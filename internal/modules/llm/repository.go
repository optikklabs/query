package llm

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/llm/pricing"
	"github.com/optikklabs/query/internal/shared/chargs"
)

const rollupTable = "optikk.llm_stats_1m"

const latencyOps = "('chat', 'agent')"

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ModelUsage(ctx context.Context, tenantID, startMs, endMs int64) ([]modelUsageRow, error) {
	query := `
		SELECT gen_ai_request_model AS model,
		       argMax(gen_ai_system, (timestamp, service, gen_ai_system)) AS vendor,
		       sum(span_count)      AS traces,
		       sum(input_tokens)    AS in_tokens,
		       sum(output_tokens)   AS out_tokens,
		       quantilesTDigestMergeIf(0.5, 0.95, 0.99)(latency_state, gen_ai_operation IN ` + latencyOps + `) AS qs,
		       sum(` + pricing.RollupCostSQL + `) AS cost
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE gen_ai_request_model != ''
		GROUP BY model
		ORDER BY cost DESC, model ASC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	var rows []modelUsageRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "llm.ModelUsage", &rows, query, args...)
}

func (r *Repository) OverviewWindows(ctx context.Context, tenantID, startMs, endMs int64) ([]overviewWindowRow, error) {
	query := `
		SELECT if(timestamp >= @start, 1, 0) AS is_current,
		       sumIf(span_count, gen_ai_operation = 'chat' AND gen_ai_request_model != '') AS llm_spans,
		       sumIf(span_count, gen_ai_operation = 'tool') AS tool_spans,
		       sum(span_count)    AS total_spans,
		       sum(error_count)   AS error_spans,
		       sum(input_tokens)  AS in_tokens,
		       sum(output_tokens) AS out_tokens,
		       quantilesTDigestMergeIf(0.5, 0.95, 0.99)(latency_state, gen_ai_operation IN ` + latencyOps + `) AS qs,
		       sum(` + pricing.RollupCostSQL + `) AS cost
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID AND timestamp >= @prevStart AND timestamp < @end
		GROUP BY is_current`
	args := append(overviewArgs(tenantID, startMs, endMs), pricing.Args()...)
	var rows []overviewWindowRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "llm.OverviewWindows", &rows, query, args...)
}

func (r *Repository) OverviewSeries(ctx context.Context, tenantID, startMs, endMs int64) ([]overviewSeriesRow, error) {
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       sumIf(span_count, gen_ai_operation = 'chat' AND gen_ai_request_model != '') AS llm_spans,
		       sumIf(span_count, gen_ai_operation = 'tool') AS tool_spans,
		       sum(span_count)  AS total_spans,
		       sum(error_count) AS error_spans,
		       quantilesTDigestMergeIf(0.5, 0.95, 0.99)(latency_state, gen_ai_operation IN ` + latencyOps + `) AS qs,
		       sum(` + pricing.RollupCostSQL + `) AS cost
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	var rows []overviewSeriesRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "llm.OverviewSeries", &rows, query, args...)
}

func (r *Repository) TraceCounts(ctx context.Context, tenantID, startMs, endMs int64) ([]traceCountRow, error) {
	query := `
		SELECT if(timestamp >= @start, 1, 0) AS is_current,
		       uniqExact(trace_id) AS traces,
		       count()        AS spans
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID AND timestamp >= @prevStart AND timestamp < @end
		WHERE is_gen_ai
		GROUP BY is_current`
	var rows []traceCountRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "llm.TraceCounts", &rows, query,
		overviewArgs(tenantID, startMs, endMs)...)
}

func overviewArgs(tenantID, startMs, endMs int64) []any {
	prevStartMs := startMs - (endMs - startMs)
	return append(chargs.RangeArgs(tenantID, startMs, endMs),
		clickhouse.Named("prevStart", time.UnixMilli(prevStartMs)))
}
