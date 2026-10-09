package repository

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	"github.com/optikklabs/query/internal/shared/spanstats"
)

const (
	DefaultExecutionsLimit = 50
	MaxExecutionsLimit     = 200
)

type SummaryRaw struct {
	QueryText      string    `ch:"query_text"`
	DbSystem       string    `ch:"db_system_any"`
	CollectionName string    `ch:"collection_name"`
	CallCount      uint64    `ch:"call_count"`
	ErrorCount     uint64    `ch:"error_count"`
	QS             []float64 `ch:"qs"`
	TotalTimeMs    float64   `ch:"total_time_ms"`
	AvgRows        *float64  `ch:"avg_rows"`
}

// GetSummary aggregates the query hash over the window from the span
// rollups; a hash with no executions yields a zero CallCount.
func (r *Repository) GetSummary(ctx context.Context, tenantID, startMs, endMs int64, hash string, f filter.Filters) (SummaryRaw, error) {
	filterWhere, filterArgs := filter.BuildSpanClauses(f)
	query := `
		SELECT any(db_statement)                                     AS query_text,
		       argMax(db_system, timestamp)                          AS db_system_any,
		       argMax(db_name, timestamp)                            AS collection_name,
		       sum(request_count)                                    AS call_count,
		       sumIf(request_count, ` + spanstats.ErrorPred + `)     AS error_count,
		       quantilesTDigestMerge(0.5, 0.95, 0.99)(latency_state) AS qs,
		       sum(duration_ms_sum)                                  AS total_time_ms,
		       sum(db_rows_sum) / nullIf(sum(db_rows_count), 0)      AS avg_rows
		FROM ` + timebucket.SpanStatsRollup(startMs, endMs) + queryHashPrewhere + filterWhere

	args := append(hashArgs(tenantID, startMs, endMs, hash), filterArgs...)
	var row SummaryRaw
	err := dbutil.QueryRowCH(dbutil.OverviewCtx(ctx), r.db, "querydetail.GetSummary", &row, query, args...)
	return row, err
}

type ServiceRaw struct {
	Service   string `ch:"service"`
	CallCount uint64 `ch:"call_count"`
}

func (r *Repository) GetServices(ctx context.Context, tenantID, startMs, endMs int64, hash string, f filter.Filters) ([]ServiceRaw, error) {
	filterWhere, filterArgs := filter.BuildSpanClauses(f)
	query := `
		SELECT service, sum(request_count) AS call_count
		FROM ` + timebucket.SpanStatsRollup(startMs, endMs) + queryHashPrewhere + filterWhere + `
		GROUP BY service
		ORDER BY call_count DESC, service ASC
		LIMIT 10`

	args := append(hashArgs(tenantID, startMs, endMs, hash), filterArgs...)
	var rows []ServiceRaw
	err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "querydetail.GetServices", &rows, query, args...)
	return rows, err
}

type TimeseriesRaw struct {
	BucketAt   time.Time `ch:"bucket_at"`
	CallCount  uint64    `ch:"call_count"`
	ErrorCount uint64    `ch:"error_count"`
	AvgMs      float64   `ch:"avg_ms"`
	P99Ms      float64   `ch:"p99_ms"`
}

func (r *Repository) GetTimeseries(ctx context.Context, tenantID, startMs, endMs int64, hash string, f filter.Filters) ([]TimeseriesRaw, error) {
	filterWhere, filterArgs := filter.BuildSpanClauses(f)
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + `       AS bucket_at,
		       sum(request_count)                                  AS call_count,
		       sumIf(request_count, ` + spanstats.ErrorPred + `)   AS error_count,
		       sum(duration_ms_sum) / sum(request_count)           AS avg_ms,
		       toFloat64(quantilesTDigestMerge(0.99)(latency_state)[1])   AS p99_ms
		FROM ` + timebucket.SpanStatsRollup(startMs, endMs) + queryHashPrewhere + filterWhere + `
		GROUP BY bucket_at
		ORDER BY bucket_at`

	args := append(hashArgs(tenantID, startMs, endMs, hash), filterArgs...)
	var rows []TimeseriesRaw
	err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "querydetail.GetTimeseries", &rows, query, args...)
	return rows, err
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
		ORDER BY timestamp DESC, span_id DESC
		LIMIT @qLimit`

	args := append(hashArgs(tenantID, startMs, endMs, hash), clickhouse.Named("qLimit", uint64(limit)))
	args = append(args, filterArgs...)
	var rows []ExecutionRaw
	err := dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "querydetail.GetExecutions", &rows, query, args...)
	return rows, err
}
