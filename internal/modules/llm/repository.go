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
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		GROUP BY service
		ORDER BY llm_spans DESC`
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
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		WHERE gen_ai_request_model != ''
		GROUP BY service, vendor, model
		ORDER BY cost DESC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	var rows []modelBreakdownRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "llm.ModelBreakdown", &rows, query, args...)
}

func (r *Repository) ModelUsage(ctx context.Context, tenantID, startMs, endMs int64) ([]modelUsageRow, error) {
	query := `
		SELECT gen_ai_request_model AS model,
		       any(gen_ai_system)   AS vendor,
		       sum(span_count)      AS traces,
		       sum(input_tokens)    AS in_tokens,
		       sum(output_tokens)   AS out_tokens,
		       quantilesTDigestMergeIf(0.5, 0.95, 0.99)(latency_state, gen_ai_operation IN ` + latencyOps + `) AS qs,
		       sum(` + pricing.TokenCostSQL("input_tokens", "output_tokens", "gen_ai_request_model") + `) AS cost
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		WHERE gen_ai_request_model != ''
		GROUP BY model
		ORDER BY cost DESC`
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
		       sum(` + pricing.TokenCostSQL("input_tokens", "output_tokens", "gen_ai_request_model") + `) AS value
		FROM ` + rollupTable + `
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
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
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
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
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @prevStart AND @end
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
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		GROUP BY bucket_at
		ORDER BY bucket_at ASC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	var rows []overviewSeriesRow
	return rows, dbutil.SelectCH(dbutil.OverviewCtx(ctx), r.db, "llm.OverviewSeries", &rows, query, args...)
}

func (r *Repository) TraceCounts(ctx context.Context, tenantID, startMs, endMs int64) ([]traceCountRow, error) {
	query := `
		SELECT if(timestamp >= @start, 1, 0) AS is_current,
		       uniqCombined64(trace_id) AS traces,
		       count()        AS spans
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @prevStart AND @end
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

func (r *Repository) QueryTraces(ctx context.Context, tenantID int64, req TracesQueryRequest) ([]llmTraceRow, error) {
	where, args := buildTraceFilters(tenantID, req)
	cur, _ := decodeTraceCursor(req.Cursor)
	if cur.SpanID != "" {
		where += ` AND (s.timestamp, s.span_id) < (@curStart, @curSpanID)`
		args = append(args,

			clickhouse.DateNamed("curStart", time.Unix(0, int64(cur.StartNs)), clickhouse.NanoSeconds),
			clickhouse.Named("curSpanID", cur.SpanID),
		)
	}
	args = append(args, clickhouse.Named("pgLimit", uint64(req.Limit+1)))
	args = append(args, pricing.Args()...)

	// Bound the aggregation scan to the requested services (fix 3.8).
	cteServiceFilter := ""
	if len(req.Services) > 0 {
		cteServiceFilter = `
		         AND service IN @services`
	}

	query := `
		WITH llm AS (
		    SELECT trace_id,
		           sum(gen_ai_input_tokens)  AS input_tokens,
		           sum(gen_ai_output_tokens) AS output_tokens,
		           anyIf(gen_ai_system, gen_ai_system != '') AS vendor,
		           argMaxIf(gen_ai_request_model, gen_ai_input_tokens + gen_ai_output_tokens, gen_ai_request_model != '') AS model,
		           countIf(gen_ai_operation = 'chat' AND gen_ai_request_model != '') AS llm_calls,
		           anyIf(substring(gen_ai_prompt, 1, 160), gen_ai_prompt != '') AS prompt_preview,
		           sum(` + pricing.TokenCostSQL("gen_ai_input_tokens", "gen_ai_output_tokens", "gen_ai_request_model") + `) AS cost
		    FROM optikk.spans
		    PREWHERE tenant_id = @tenantID
		         AND timestamp BETWEEN @start AND @end
		         AND is_gen_ai` + cteServiceFilter + `
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
		       s.llm_user_id        AS user_id,
		       s.llm_session_id     AS session_id,
		       s.llm_tags           AS tags,
		       llm.vendor           AS vendor,
		       llm.model            AS model,
		       llm.llm_calls        AS llm_calls,
		       llm.prompt_preview   AS prompt_preview,
		       llm.input_tokens     AS input_tokens,
		       llm.output_tokens    AS output_tokens,
		       llm.cost             AS cost
		FROM optikk.spans AS s
		INNER JOIN llm ON s.trace_id = llm.trace_id
		PREWHERE s.tenant_id = @tenantID AND s.timestamp BETWEEN @start AND @end AND s.is_root = 1
		WHERE 1=1` + where + `
		ORDER BY s.timestamp DESC, s.span_id DESC
		LIMIT @pgLimit`

	var rows []llmTraceRow
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "llm.QueryTraces", &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
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

// Payload caps for trace detail: big agent traces would otherwise return
// full prompt/completion text for every span (multi-MB responses).
const (
	traceSpansMaxRows   = 2000
	traceSpanIOMaxChars = 4096
)

func (r *Repository) TraceSpans(ctx context.Context, tenantID int64, traceID string, startTimeMs, endTimeMs int64) ([]traceSpanRow, error) {
	query := `
		SELECT span_id, parent_span_id, timestamp, duration_nano, name, service, environment,
		       gen_ai_system, gen_ai_operation, gen_ai_span_kind,
		       gen_ai_request_model, gen_ai_response_model,
		       gen_ai_input_tokens, gen_ai_output_tokens, has_error,
		       llm_user_id, llm_session_id, llm_release,
		       leftUTF8(gen_ai_prompt, @ioMaxChars)              AS prompt,
		       lengthUTF8(gen_ai_prompt) > @ioMaxChars           AS prompt_truncated,
		       leftUTF8(gen_ai_completion, @ioMaxChars)          AS completion,
		       lengthUTF8(gen_ai_completion) > @ioMaxChars       AS completion_truncated
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND trace_id = @traceID
		ORDER BY timestamp ASC
		LIMIT @maxSpans`
	var rows []traceSpanRow
	args := append(chargs.RangeArgs(tenantID, startTimeMs, endTimeMs),
		clickhouse.Named("traceID", traceID),
		clickhouse.Named("ioMaxChars", uint64(traceSpanIOMaxChars)),
		clickhouse.Named("maxSpans", uint64(traceSpansMaxRows)),
	)
	return rows, dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "llm.TraceSpans", &rows, query,
		args...,
	)
}

// TraceSpanIO fetches the untruncated prompt/completion for a single span.
func (r *Repository) TraceSpanIO(ctx context.Context, tenantID int64, traceID, spanID string, startTimeMs, endTimeMs int64) (spanIORow, bool, error) {
	query := `
		SELECT gen_ai_prompt     AS prompt,
		       gen_ai_completion AS completion
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp BETWEEN @start AND @end
		     AND trace_id = @traceID
		WHERE span_id = @spanID
		LIMIT 1`
	args := append(chargs.RangeArgs(tenantID, startTimeMs, endTimeMs),
		clickhouse.Named("traceID", traceID),
		clickhouse.Named("spanID", spanID),
	)
	var rows []spanIORow
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "llm.TraceSpanIO", &rows, query, args...); err != nil {
		return spanIORow{}, false, err
	}
	if len(rows) == 0 {
		return spanIORow{}, false, nil
	}
	return rows[0], true, nil
}

func (r *Repository) ScoresForTraces(ctx context.Context, tenantID, startMs, endMs int64, traceIDs []string) ([]traceScoreRow, error) {
	if len(traceIDs) == 0 {
		return nil, nil
	}
	query := `
		SELECT trace_id, name, data_type, value, string_value, source, comment
		FROM optikk.llm_scores
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		WHERE trace_id IN @traceIDs
		ORDER BY timestamp ASC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), clickhouse.Named("traceIDs", traceIDs))
	var rows []traceScoreRow
	return rows, dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "llm.ScoresForTraces", &rows, query, args...)
}
