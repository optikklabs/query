package repository

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
)

const DefaultExecutionsLimit = 50

const maxExecutionsLimit = 200

func clampExecutionsLimit(limit int) int {
	if limit <= 0 {
		return DefaultExecutionsLimit
	}
	return min(limit, maxExecutionsLimit)
}

type SummaryRaw struct {
	QueryText      string    `ch:"query_text"`
	DbSystem       string    `ch:"db_system_any"`
	CollectionName string    `ch:"collection_name"`
	OperationName  string    `ch:"operation_name"`
	CallCount      uint64    `ch:"call_count"`
	ErrorCount     uint64    `ch:"error_count"`
	QS             []float32 `ch:"qs"`
	AvgMs          float64   `ch:"avg_ms"`
	TotalTimeMs    float64   `ch:"total_time_ms"`
	AvgRows        *float64  `ch:"avg_rows"`
}

func (r *Repository) GetSummary(ctx context.Context, tenantID, startMs, endMs int64, hash string, f filter.Filters) (*SummaryRaw, error) {
	filterWhere, filterArgs := filter.BuildSpanClauses(f)
	query := `
		SELECT any(db_statement_normalized)                       AS query_text,
		       any(db_system)                                     AS db_system_any,
		       any(db_name)                                       AS collection_name,
		       any(attributes['db.operation.name'])               AS operation_name,
		       count()                                            AS call_count,
		       countIf(is_error)                                  AS error_count,
		       quantilesTiming(0.5, 0.95, 0.99)(duration_nano / 1000000.0) AS qs,
		       avg(duration_nano / 1000000.0)                     AS avg_ms,
		       sum(duration_nano) / 1000000.0                     AS total_time_ms,
		       avgOrNull(toFloat64OrNull(attributes['db.response.returned_rows'])) AS avg_rows
		FROM optikk.spans` + queryHashPrewhere + filterWhere

	args := append(hashArgs(tenantID, startMs, endMs, hash), filterArgs...)
	var rows []SummaryRaw
	if err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "querydetail.GetSummary", &rows, query, args...); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

type ServiceRaw struct {
	Service   string `ch:"service"`
	CallCount uint64 `ch:"call_count"`
}

func (r *Repository) GetServices(ctx context.Context, tenantID, startMs, endMs int64, hash string, f filter.Filters) ([]ServiceRaw, error) {
	filterWhere, filterArgs := filter.BuildSpanClauses(f)
	query := `
		SELECT service, count() AS call_count
		FROM optikk.spans` + queryHashPrewhere + filterWhere + `
		GROUP BY service
		ORDER BY call_count DESC
		LIMIT 10`

	args := append(hashArgs(tenantID, startMs, endMs, hash), filterArgs...)
	var rows []ServiceRaw
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "querydetail.GetServices", &rows, query, args...)
}

type TimeseriesRaw struct {
	BucketAt   time.Time `ch:"bucket_at"`
	CallCount  uint64    `ch:"call_count"`
	ErrorCount uint64    `ch:"error_count"`
	AvgMs      float64   `ch:"avg_ms"`
	P99Ms      float32   `ch:"p99_ms"`
}

func (r *Repository) GetTimeseries(ctx context.Context, tenantID, startMs, endMs int64, hash string, f filter.Filters) ([]TimeseriesRaw, error) {
	filterWhere, filterArgs := filter.BuildSpanClauses(f)
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       count()                                           AS call_count,
		       countIf(is_error)                                 AS error_count,
		       avg(duration_nano / 1000000.0)                    AS avg_ms,
		       quantileTiming(0.99)(duration_nano / 1000000.0)   AS p99_ms
		FROM optikk.spans` + queryHashPrewhere + filterWhere + `
		GROUP BY bucket_at
		ORDER BY bucket_at`

	args := append(hashArgs(tenantID, startMs, endMs, hash), filterArgs...)
	var rows []TimeseriesRaw
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "querydetail.GetTimeseries", &rows, query, args...)
}

type ExecutionRaw struct {
	Timestamp  time.Time `ch:"timestamp"`
	TraceID    string    `ch:"trace_id"`
	SpanID     string    `ch:"span_id"`
	DurationMs float64   `ch:"duration_ms"`
	IsError    uint8     `ch:"is_err"`
	Service    string    `ch:"service"`
	Host       string    `ch:"host"`
	Rows       *float64  `ch:"row_count"`
}

func (r *Repository) GetExecutions(ctx context.Context, tenantID, startMs, endMs int64, hash string, f filter.Filters, limit int) ([]ExecutionRaw, error) {
	limit = clampExecutionsLimit(limit)
	filterWhere, filterArgs := filter.BuildSpanClauses(f)
	query := `
		SELECT timestamp,
		       trace_id,
		       span_id,
		       duration_nano / 1000000.0 AS duration_ms,
		       is_error                  AS is_err,
		       service,
		       host,
		       toFloat64OrNull(attributes['db.response.returned_rows']) AS row_count
		FROM optikk.spans` + queryHashPrewhere + filterWhere + `
		ORDER BY timestamp DESC
		LIMIT @qLimit`

	args := append(hashArgs(tenantID, startMs, endMs, hash), clickhouse.Named("qLimit", uint64(limit)))
	args = append(args, filterArgs...)
	var rows []ExecutionRaw
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "querydetail.GetExecutions", &rows, query, args...)
}
