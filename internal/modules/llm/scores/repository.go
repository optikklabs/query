package scores

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/shared/chargs"
)

const scoresTable = "optikk.llm_scores"

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

type scoreInsert struct {
	TenantID    int64
	TraceID     string
	SpanID      string
	SessionID   string
	UserID      string
	Service     string
	Environment string
	Name        string
	DataType    string
	Value       float64
	StringValue string
	Comment     string
}

// traceLookbackMs bounds how far back a scored trace is searched for.
const traceLookbackMs = int64(30 * 24 * time.Hour / time.Millisecond)

// LookupTraceContext copies the scored trace's service, environment,
// session and user onto the score; sql.ErrNoRows means no such trace.
func (r *Repository) LookupTraceContext(ctx context.Context, tenantID int64, traceID string) (scoreInsert, error) {
	nowMs := time.Now().UnixMilli()
	query := `
		SELECT argMax(service, (timestamp, span_id)) AS service_any,
		       argMax(environment, (timestamp, span_id)) AS environment_any,
		       argMaxIf(llm_session_id, (timestamp, span_id), llm_session_id != '') AS session_id,
		       argMaxIf(llm_user_id, (timestamp, span_id), llm_user_id != '') AS user_id
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND trace_id = @traceID
		HAVING count() > 0`
	var row struct {
		Service     string `ch:"service_any"`
		Environment string `ch:"environment_any"`
		SessionID   string `ch:"session_id"`
		UserID      string `ch:"user_id"`
	}
	args := append(chargs.RangeArgs(tenantID, nowMs-traceLookbackMs, nowMs), clickhouse.Named("traceID", traceID))
	if err := dbutil.QueryRowCH(dbutil.ExplorerCtx(ctx), r.db, "llm.scores.LookupTraceContext", &row, query, args...); err != nil {
		return scoreInsert{}, err
	}
	return scoreInsert{
		Service: row.Service, Environment: row.Environment,
		SessionID: row.SessionID, UserID: row.UserID,
	}, nil
}

func (r *Repository) Insert(ctx context.Context, s scoreInsert) error {
	query := `INSERT INTO ` + scoresTable + `
		(tenant_id, timestamp, trace_id, span_id, session_id, user_id, service,
		 environment, name, source, data_type, value, string_value, comment, evaluator_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'human', ?, ?, ?, ?, 0)`
	return r.db.Exec(ctx, query,
		uint32(s.TenantID), time.Now(), s.TraceID, s.SpanID, s.SessionID, s.UserID,
		s.Service, s.Environment, s.Name, s.DataType, s.Value, s.StringValue, s.Comment)
}

func (r *Repository) Summary(ctx context.Context, tenantID, startMs, endMs int64) ([]summaryRow, error) {
	query := `
		SELECT name, argMax(data_type, (timestamp, trace_id, span_id)) AS data_type,
		       count() AS cnt, avg(value) AS mean
		FROM ` + scoresTable + `
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		GROUP BY name
		ORDER BY cnt DESC, name ASC`
	var rows []summaryRow
	err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "llm.scores.Summary", &rows, query,
		chargs.RangeArgs(tenantID, startMs, endMs)...)
	return rows, err
}

// MeanColumn is the llm_scores column MeanScores groups by.
type MeanColumn string

const (
	BySession MeanColumn = "session_id"
	ByUser    MeanColumn = "user_id"
)

// MeanScores returns the mean numeric score of each of ids in column col.
// Ids with no numeric score are absent from the map.
func MeanScores(ctx context.Context, db clickhouse.Conn, op string, col MeanColumn, tenantID, startMs, endMs int64, ids []string) (map[string]*float64, error) {
	query := `
		SELECT ` + string(col) + ` AS id, avg(value) AS mean
		FROM ` + scoresTable + `
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE ` + string(col) + ` IN @ids AND data_type = 'numeric'
		GROUP BY id`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), clickhouse.Named("ids", ids))
	var rows []struct {
		ID   string  `ch:"id"`
		Mean float64 `ch:"mean"`
	}
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), db, op, &rows, query, args...); err != nil {
		return nil, err
	}
	means := make(map[string]*float64, len(rows))
	for _, row := range rows {
		means[row.ID] = &row.Mean
	}
	return means, nil
}
