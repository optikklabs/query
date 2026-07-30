package llm

import (
	"context"
	"strconv"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/llm/pricing"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

func (r *Repository) QueryTraces(ctx context.Context, tenantID int64, req TracesQueryRequest) ([]llmTraceRow, error) {
	// Vendor/model filters apply to the per-trace aggregate, so they force
	// aggregating every trace in range before paging. Without them, page the
	// root spans first and aggregate only the page's traces.
	if len(req.Vendors) > 0 || len(req.Models) > 0 {
		return r.queryTracesPreAggregated(ctx, tenantID, req)
	}

	roots, err := r.queryTraceRootPage(ctx, tenantID, req)
	if err != nil || len(roots) == 0 {
		return roots, err
	}
	traceIDs := make([]string, len(roots))
	for i, row := range roots {
		traceIDs[i] = row.TraceID
	}
	aggs, err := r.traceAggregates(ctx, tenantID, req.StartTime, req.EndTime, traceIDs)
	if err != nil {
		return nil, err
	}
	aggByTrace := make(map[string]llmTraceRow, len(aggs))
	for _, agg := range aggs {
		aggByTrace[agg.TraceID] = agg
	}
	for i := range roots {
		agg := aggByTrace[roots[i].TraceID]
		roots[i].Vendor = agg.Vendor
		roots[i].Model = agg.Model
		roots[i].LLMCalls = agg.LLMCalls
		roots[i].PromptPreview = agg.PromptPreview
		roots[i].InputTokens = agg.InputTokens
		roots[i].OutputTokens = agg.OutputTokens
		roots[i].Cost = agg.Cost
	}
	return roots, nil
}

func (r *Repository) queryTraceRootPage(ctx context.Context, tenantID int64, req TracesQueryRequest) ([]llmTraceRow, error) {
	where, args := buildTraceFilters(tenantID, req)
	where, args = appendCursorFilter(where, args, req.Cursor)
	args = append(args, clickhouse.Named("pgLimit", uint64(req.Limit+1)))

	genAIServiceFilter := ""
	if len(req.Services) > 0 {
		genAIServiceFilter = ` AND service IN @services`
	}

	query := `
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
		       s.llm_tags           AS tags
		FROM optikk.spans AS s
		PREWHERE s.tenant_id = @tenantID AND s.timestamp BETWEEN @start AND @end AND s.is_root = 1
		WHERE s.trace_id IN (
		    SELECT trace_id
		    FROM optikk.spans
		    PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND is_gen_ai` + genAIServiceFilter + `
		    GROUP BY trace_id
		    ORDER BY max(timestamp) DESC
		    LIMIT ` + strconv.Itoa(filterutil.MaxMatchedTraces) + `
		)` + where + `
		ORDER BY s.timestamp DESC, s.span_id DESC
		LIMIT @pgLimit`

	var rows []llmTraceRow
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "llm.QueryTraces.RootPage", &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) traceAggregates(ctx context.Context, tenantID, startMs, endMs int64, traceIDs []string) ([]llmTraceRow, error) {
	query := `
		SELECT trace_id,
		       sum(gen_ai_input_tokens)  AS input_tokens,
		       sum(gen_ai_output_tokens) AS output_tokens,
		       anyIf(gen_ai_system, gen_ai_system != '') AS vendor,
		       argMaxIf(gen_ai_request_model, gen_ai_input_tokens + gen_ai_output_tokens, gen_ai_request_model != '') AS model,
		       countIf(gen_ai_operation = 'chat' AND gen_ai_request_model != '') AS llm_calls,
		       anyIf(substring(gen_ai_prompt, 1, 160), gen_ai_prompt != '') AS prompt_preview,
		       sum(` + pricing.TokenCostSQL("gen_ai_input_tokens", "gen_ai_output_tokens", "gen_ai_request_model") + `) AS cost
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end AND is_gen_ai
		WHERE trace_id IN @traceIDs
		GROUP BY trace_id`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	args = append(args, clickhouse.Named("traceIDs", traceIDs))
	var rows []llmTraceRow
	if err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "llm.QueryTraces.Aggregates", &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *Repository) queryTracesPreAggregated(ctx context.Context, tenantID int64, req TracesQueryRequest) ([]llmTraceRow, error) {
	where, args := buildTraceFilters(tenantID, req)
	where, args = appendCursorFilter(where, args, req.Cursor)
	args = append(args, clickhouse.Named("pgLimit", uint64(req.Limit+1)))
	args = append(args, pricing.Args()...)

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

func appendCursorFilter(where string, args []any, rawCursor string) (string, []any) {
	cur, _ := decodeTraceCursor(rawCursor)
	if cur.SpanID == "" {
		return where, args
	}
	where += ` AND (s.timestamp, s.span_id) < (@curStart, @curSpanID)`
	args = append(args,
		clickhouse.DateNamed("curStart", time.Unix(0, int64(cur.StartNs)), clickhouse.NanoSeconds),
		clickhouse.Named("curSpanID", cur.SpanID),
	)
	return where, args
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
