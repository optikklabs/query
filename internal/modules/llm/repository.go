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

func (r *Repository) AppAggregates(ctx context.Context, tenantID, startMs, endMs int64) ([]appAggRow, error) {
	query := `
		SELECT service,
		       sumIf(span_count, gen_ai_operation = 'chat' AND gen_ai_request_model != '') AS llm_spans,
		       sumIf(span_count, gen_ai_operation = 'tool')      AS tool_spans,
		       sumIf(span_count, gen_ai_operation = 'retrieval') AS retrieval_spans,
		       sumIf(span_count, gen_ai_operation = 'embedding') AS embedding_spans,
		       sumIf(span_count, gen_ai_operation = 'agent')     AS agent_spans,
		       sum(span_count)   AS total_spans,
		       sum(error_count)  AS error_spans,
		       sum(input_tokens) AS in_tokens,
		       sum(output_tokens) AS out_tokens,
		       quantilesTDigestMergeIf(0.5, 0.95, 0.99)(latency_state, gen_ai_operation IN ` + latencyOps + `) AS qs,
		       sum(` + pricing.TokenCostSQL("input_tokens", "output_tokens", "gen_ai_request_model") + `) AS cost
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		GROUP BY service
		ORDER BY llm_spans DESC, service ASC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	var rows []appAggRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "llm.AppAggregates", &rows, query, args...)
}

func (r *Repository) ModelBreakdown(ctx context.Context, tenantID, startMs, endMs int64) ([]modelBreakdownRow, error) {
	query := `
		SELECT service,
		       gen_ai_system        AS vendor,
		       gen_ai_request_model AS model,
		       sum(span_count)      AS spans,
		       sum(input_tokens)    AS in_tokens,
		       sum(output_tokens)   AS out_tokens,
		       sum(` + pricing.TokenCostSQL("input_tokens", "output_tokens", "gen_ai_request_model") + `) AS cost
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE gen_ai_request_model != ''
		GROUP BY service, vendor, model
		ORDER BY cost DESC, service ASC, vendor ASC, model ASC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	var rows []modelBreakdownRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "llm.ModelBreakdown", &rows, query, args...)
}

func costBreakdownKeyColumn(groupBy string) string {
	switch groupBy {
	case "vendor":
		return "vendor"
	case "model":
		return "model"
	default:
		return "service"
	}
}

func (r *Repository) CostBreakdownByKey(ctx context.Context, tenantID, startMs, endMs int64, groupBy string) ([]costBreakdownRow, error) {
	key := costBreakdownKeyColumn(groupBy)
	// Inner grouping mirrors ModelBreakdown so the dominant vendor is the
	// vendor of the largest (service, vendor, model) slice within each key.
	query := `
		SELECT ` + key + `                  AS key,
		       argMax(vendor, span_total)   AS top_vendor,
		       sum(span_total)              AS spans,
		       sum(in_tok)                  AS in_tokens,
		       sum(out_tok)                 AS out_tokens,
		       sum(cost_total)              AS cost
		FROM (
		    SELECT service,
		           gen_ai_system        AS vendor,
		           gen_ai_request_model AS model,
		           sum(span_count)      AS span_total,
		           sum(input_tokens)    AS in_tok,
		           sum(output_tokens)   AS out_tok,
		           sum(` + pricing.TokenCostSQL("input_tokens", "output_tokens", "gen_ai_request_model") + `) AS cost_total
		    FROM ` + rollupTable + `
		    PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		    WHERE gen_ai_request_model != ''
		    GROUP BY service, vendor, model
		)
		GROUP BY key
		ORDER BY cost DESC, key ASC
		LIMIT 1000`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	var rows []costBreakdownRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "llm.CostBreakdownByKey", &rows, query, args...)
}

func (r *Repository) ModelUsage(ctx context.Context, tenantID, startMs, endMs int64) ([]modelUsageRow, error) {
	query := `
		SELECT gen_ai_request_model AS model,
		       argMax(gen_ai_system, (timestamp, service, gen_ai_system)) AS vendor,
		       sum(span_count)      AS traces,
		       sum(input_tokens)    AS in_tokens,
		       sum(output_tokens)   AS out_tokens,
		       quantilesTDigestMergeIf(0.5, 0.95, 0.99)(latency_state, gen_ai_operation IN ` + latencyOps + `) AS qs,
		       sum(` + pricing.TokenCostSQL("input_tokens", "output_tokens", "gen_ai_request_model") + `) AS cost
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE gen_ai_request_model != ''
		GROUP BY model
		ORDER BY cost DESC, model ASC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	var rows []modelUsageRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "llm.ModelUsage", &rows, query, args...)
}

func (r *Repository) AppTrends(ctx context.Context, tenantID, startMs, endMs int64) ([]trendRow, error) {
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       service,
		       sum(span_count) AS cnt
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		GROUP BY bucket_at, service
		ORDER BY bucket_at ASC`
	var rows []trendRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "llm.AppTrends", &rows, query,
		chargs.RangeArgs(tenantID, startMs, endMs)...)
}

func (r *Repository) TokensByVendor(ctx context.Context, tenantID, startMs, endMs int64) ([]keyedBucketRow, error) {
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       gen_ai_system AS key,
		       toFloat64(sum(input_tokens + output_tokens)) AS value
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE gen_ai_system != ''
		GROUP BY bucket_at, key
		ORDER BY bucket_at ASC`
	var rows []keyedBucketRow
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "llm.TokensByVendor", &rows, query,
		chargs.RangeArgs(tenantID, startMs, endMs)...)
}

func (r *Repository) SpendByVendor(ctx context.Context, tenantID, startMs, endMs int64) ([]keyedBucketRow, error) {
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       gen_ai_system AS key,
		       sum(` + pricing.TokenCostSQL("input_tokens", "output_tokens", "gen_ai_request_model") + `) AS value
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE gen_ai_request_model != ''
		GROUP BY bucket_at, key
		ORDER BY bucket_at ASC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	var rows []keyedBucketRow
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "llm.SpendByVendor", &rows, query, args...)
}

func (r *Repository) LatencyPercentiles(ctx context.Context, tenantID, startMs, endMs int64) ([]latencyBucketRow, error) {
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       quantilesTDigestMergeIf(0.5, 0.95, 0.99)(latency_state, gen_ai_operation IN ` + latencyOps + `) AS qs
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`
	var rows []latencyBucketRow
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "llm.LatencyPercentiles", &rows, query,
		chargs.RangeArgs(tenantID, startMs, endMs)...)
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
		       sum(` + pricing.TokenCostSQL("input_tokens", "output_tokens", "gen_ai_request_model") + `) AS cost
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
		       sum(` + pricing.TokenCostSQL("input_tokens", "output_tokens", "gen_ai_request_model") + `) AS cost
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
