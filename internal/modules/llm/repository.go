package llm

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
)

const rollupTable = "optikk.llm_stats_1m"

// latencyOps: end-to-end latency comes from the request-level spans, not
// tool/embedding children.
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
		       sum(` + tokenCostSQL("input_tokens", "output_tokens", "gen_ai_request_model") + `) AS cost
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		GROUP BY service
		ORDER BY llm_spans DESC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), priceArgs()...)
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
		       sum(` + tokenCostSQL("input_tokens", "output_tokens", "gen_ai_request_model") + `) AS cost
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		WHERE gen_ai_request_model != ''
		GROUP BY service, vendor, model
		ORDER BY cost DESC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), priceArgs()...)
	var rows []modelBreakdownRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "llm.ModelBreakdown", &rows, query, args...)
}

func (r *Repository) AppTrends(ctx context.Context, tenantID, startMs, endMs int64) ([]trendRow, error) {
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       service,
		       sum(span_count) AS cnt
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
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
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
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
		       sum(` + tokenCostSQL("input_tokens", "output_tokens", "gen_ai_request_model") + `) AS value
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		WHERE gen_ai_request_model != ''
		GROUP BY bucket_at, key
		ORDER BY bucket_at ASC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), priceArgs()...)
	var rows []keyedBucketRow
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "llm.SpendByVendor", &rows, query, args...)
}

func (r *Repository) LatencyPercentiles(ctx context.Context, tenantID, startMs, endMs int64) ([]latencyBucketRow, error) {
	query := `
		SELECT ` + timebucket.DisplayGrainSQL(endMs-startMs) + ` AS bucket_at,
		       quantilesTDigestMergeIf(0.5, 0.95, 0.99)(latency_state, gen_ai_operation IN ` + latencyOps + `) AS qs
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`
	var rows []latencyBucketRow
	return rows, dbutil.SelectCH(dbutil.DashboardCtx(ctx), r.db, "llm.LatencyPercentiles", &rows, query,
		chargs.RangeArgs(tenantID, startMs, endMs)...)
}

// QueryTraces lists root spans of traces containing gen_ai spans, joined
// with per-trace token/cost totals aggregated from the gen_ai spans.
func (r *Repository) QueryTraces(ctx context.Context, tenantID int64, req TracesQueryRequest) ([]llmTraceRow, bool, error) {
	where, args := buildTraceFilters(tenantID, req)
	cur, _ := decodeTraceCursor(req.Cursor)
	if cur.SpanID != "" {
		where += ` AND (s.timestamp, s.span_id) < (@curStart, @curSpanID)`
		args = append(args,
			// DateNamed with ns scale; a plain time.Time arg truncates to seconds.
			clickhouse.DateNamed("curStart", time.Unix(0, int64(cur.StartNs)), clickhouse.NanoSeconds),
			clickhouse.Named("curSpanID", cur.SpanID),
		)
	}
	args = append(args, clickhouse.Named("pgLimit", uint64(req.Limit+1)))
	args = append(args, priceArgs()...)

	query := `
		WITH llm AS (
		    SELECT trace_id,
		           sum(gen_ai_input_tokens)  AS input_tokens,
		           sum(gen_ai_output_tokens) AS output_tokens,
		           anyIf(gen_ai_system, gen_ai_system != '') AS vendor,
		           argMaxIf(gen_ai_request_model, gen_ai_input_tokens + gen_ai_output_tokens, gen_ai_request_model != '') AS model,
		           countIf(gen_ai_operation = 'chat' AND gen_ai_request_model != '') AS llm_calls,
		           sum(` + tokenCostSQL("gen_ai_input_tokens", "gen_ai_output_tokens", "gen_ai_request_model") + `) AS cost
		    FROM optikk.spans
		    PREWHERE tenant_id = @tenantID
		    WHERE timestamp BETWEEN @start AND @end AND is_gen_ai
		    GROUP BY trace_id
		)
		SELECT s.trace_id           AS trace_id,
		       s.span_id            AS span_id,
		       s.timestamp          AS start_time,
		       s.duration_nano      AS duration_nano,
		       s.service            AS service,
		       s.name               AS operation,
		       s.status_code_string AS status,
		       s.has_error          AS has_error,
		       llm.vendor           AS vendor,
		       llm.model            AS model,
		       llm.llm_calls        AS llm_calls,
		       llm.input_tokens     AS input_tokens,
		       llm.output_tokens    AS output_tokens,
		       llm.cost             AS cost
		FROM optikk.spans AS s
		INNER JOIN llm ON s.trace_id = llm.trace_id
		PREWHERE s.tenant_id = @tenantID
		WHERE s.timestamp BETWEEN @start AND @end AND s.is_root = 1` + where + `
		ORDER BY s.timestamp DESC, s.span_id DESC
		LIMIT @pgLimit`

	var rows []llmTraceRow
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "llm.QueryTraces", &rows, query, args...); err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > req.Limit
	if hasMore {
		rows = rows[:req.Limit]
	}
	return rows, hasMore, nil
}

func buildTraceFilters(tenantID int64, req TracesQueryRequest) (string, []any) {
	args := chargs.RangeArgs(tenantID, req.StartTime, req.EndTime)
	where := ""
	if len(req.Services) > 0 {
		where += ` AND s.service IN @services`
		args = append(args, clickhouse.Named("services", req.Services))
	}
	if len(req.Vendors) > 0 {
		where += ` AND llm.vendor IN @vendors`
		args = append(args, clickhouse.Named("vendors", req.Vendors))
	}
	if len(req.Models) > 0 {
		where += ` AND llm.model IN @models`
		args = append(args, clickhouse.Named("models", req.Models))
	}
	switch req.Status {
	case "error":
		where += ` AND s.has_error`
	case "ok":
		where += ` AND NOT s.has_error`
	}
	if req.MinDurationMs > 0 {
		where += ` AND s.duration_nano >= @minDurationNs`
		args = append(args, clickhouse.Named("minDurationNs", uint64(req.MinDurationMs*1e6)))
	}
	return where, args
}

// TraceSpans fetches every span of one trace, located via trace_index to
// keep the raw-spans scan bounded.
func (r *Repository) TraceSpans(ctx context.Context, tenantID int64, traceID string) ([]traceSpanRow, error) {
	query := `
		WITH trace_loc AS (
		    SELECT timestamp
		    FROM optikk.trace_index
		    PREWHERE trace_id = @traceID AND tenant_id = @tenantID
		    LIMIT 1
		)
		SELECT span_id, parent_span_id, timestamp, duration_nano, name, service,
		       gen_ai_system, gen_ai_operation, gen_ai_request_model,
		       gen_ai_input_tokens, gen_ai_output_tokens, has_error,
		       coalesce(toString(attributes.` + "`gen_ai.prompt`" + `), '')     AS prompt,
		       coalesce(toString(attributes.` + "`gen_ai.completion`" + `), '') AS completion
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN (SELECT timestamp FROM trace_loc) - INTERVAL 5 MINUTE
		                       AND (SELECT timestamp FROM trace_loc) + INTERVAL 24 HOUR
		     AND trace_id = @traceID
		ORDER BY timestamp ASC`
	var rows []traceSpanRow
	return rows, dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "llm.TraceSpans", &rows, query,
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("traceID", traceID),
	)
}
