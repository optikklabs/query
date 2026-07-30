package scores

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
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

func (r *Repository) LookupTraceContext(ctx context.Context, tenantID int64, traceID string) (scoreInsert, error) {
	now := time.Now()
	start := now.Add(-30 * 24 * time.Hour)
	query := `
		SELECT argMax(service, (timestamp, span_id)) AS service_any,
		       argMax(environment, (timestamp, span_id)) AS environment_any,
		       argMaxIf(llm_session_id, (timestamp, span_id), llm_session_id != '') AS session_id,
		       argMaxIf(llm_user_id, (timestamp, span_id), llm_user_id != '') AS user_id
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND trace_id = @traceID
		LIMIT 1`
	var row struct {
		Service     string `ch:"service_any"`
		Environment string `ch:"environment_any"`
		SessionID   string `ch:"session_id"`
		UserID      string `ch:"user_id"`
	}
	args := []any{
		clickhouse.Named("tenantID", tenantID),
		clickhouse.Named("traceID", traceID),
		clickhouse.Named("start", start),
		clickhouse.Named("end", now),
	}
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

func (r *Repository) Names(ctx context.Context, tenantID, startMs, endMs int64) ([]nameRow, error) {
	query := `
		SELECT name, argMax(data_type, (timestamp, trace_id, span_id)) AS data_type
		FROM ` + scoresTable + `
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		GROUP BY name
		ORDER BY name`
	var rows []nameRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "llm.scores.Names", &rows, query,
		chargs.RangeArgs(tenantID, startMs, endMs)...)
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
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "llm.scores.Summary", &rows, query,
		chargs.RangeArgs(tenantID, startMs, endMs)...)
}

func (r *Repository) Timeseries(ctx context.Context, tenantID, startMs, endMs int64, name string) ([]bucketRow, error) {
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       avg(value) AS mean
		FROM ` + scoresTable + `
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE name = @name
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), clickhouse.Named("name", name))
	var rows []bucketRow
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "llm.scores.Timeseries", &rows, query, args...)
}

func (r *Repository) Distribution(ctx context.Context, tenantID, startMs, endMs int64, name string) ([]histRow, error) {
	query := `
		SELECT least(toUInt8(value * 10), 9) AS bucket, count() AS cnt
		FROM ` + scoresTable + `
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE name = @name AND data_type = 'numeric'
		GROUP BY bucket
		ORDER BY bucket ASC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), clickhouse.Named("name", name))
	var rows []histRow
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "llm.scores.Distribution", &rows, query, args...)
}
