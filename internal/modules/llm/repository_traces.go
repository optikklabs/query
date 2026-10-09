package llm

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/llm/pricing"
	"github.com/optikklabs/query/internal/shared/chargs"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

// QueryTraces pages the root spans of traces with LLM spans, newest first,
// then aggregates the page's LLM spans. Filters on LLM spans (service, vendor,
// model) match a trace when any of its LLM spans matches.
func (r *Repository) QueryTraces(ctx context.Context, tenantID int64, req TracesQueryRequest, cur *traceCursor) ([]llmTraceRow, error) {
	roots, err := r.queryTraceRootPage(ctx, tenantID, req, cur)
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
		roots[i].HasError = roots[i].HasError || agg.LLMErrors > 0
		roots[i].Vendor = agg.Vendor
		roots[i].Model = agg.Model
		roots[i].UserID = agg.UserID
		roots[i].SessionID = agg.SessionID
		roots[i].Tags = agg.Tags
		roots[i].LLMCalls = agg.LLMCalls
		roots[i].PromptPreview = agg.PromptPreview
		roots[i].InputTokens = agg.InputTokens
		roots[i].OutputTokens = agg.OutputTokens
		roots[i].Cost = agg.Cost
	}
	return roots, nil
}

// llmSpansPrewhere scopes optikk.llm_spans to the tenant and range.
const llmSpansPrewhere = `
		    FROM optikk.llm_spans
		    PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end`

// llmErrorTraces is the set of traces with a failed LLM span.
const llmErrorTraces = `(SELECT trace_id` + llmSpansPrewhere + ` WHERE has_error)`

func (r *Repository) queryTraceRootPage(ctx context.Context, tenantID int64, req TracesQueryRequest, cur *traceCursor) ([]llmTraceRow, error) {
	llmWhere, where, args := buildTraceFilters(tenantID, req)
	where, args = appendCursorFilter(where, args, cur)
	args = append(args, clickhouse.Named("pgLimit", uint64(req.Limit+1)))

	query := `
		SELECT trace_id,
		       span_id,
		       timestamp          AS start_time,
		       duration_nano,
		       service,
		       name               AS operation,
		       status_code_string AS status,
		       has_error
		FROM optikk.spans_root
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE trace_id IN (SELECT trace_id` + llmSpansPrewhere + ` WHERE 1=1` + llmWhere + `)` + where + `
		ORDER BY timestamp DESC, span_id DESC
		LIMIT @pgLimit`

	var rows []llmTraceRow
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "llm.QueryTraces.RootPage", &rows, query, args...)
	return rows, err
}

// traceAggregates summarises the LLM spans of each trace; the identity
// fields come from LLM spans because a gateway root rarely carries them.
func (r *Repository) traceAggregates(ctx context.Context, tenantID, startMs, endMs int64, traceIDs []string) ([]llmTraceRow, error) {
	query := `
		SELECT trace_id,
		       sum(gen_ai_input_tokens)  AS input_tokens,
		       sum(gen_ai_output_tokens) AS output_tokens,
		       argMinIf(gen_ai_system, (timestamp, span_id), gen_ai_system != '') AS vendor,
		       argMaxIf(gen_ai_request_model, gen_ai_input_tokens + gen_ai_output_tokens, gen_ai_request_model != '') AS model,
		       argMinIf(llm_user_id, (timestamp, span_id), llm_user_id != '')       AS user_id,
		       argMinIf(llm_session_id, (timestamp, span_id), llm_session_id != '') AS session_id,
		       argMinIf(llm_tags, (timestamp, span_id), notEmpty(llm_tags))         AS tags,
		       countIf(gen_ai_operation = 'chat' AND gen_ai_request_model != '') AS llm_calls,
		       countIf(has_error) AS llm_errors,
		       argMinIf(prompt_preview, (timestamp, span_id), prompt_preview != '') AS prompt_preview,
		       sum(` + pricing.SpanCostSQL + `) AS cost` + llmSpansPrewhere + `
		WHERE trace_id IN @traceIDs
		GROUP BY trace_id`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), pricing.Args()...)
	args = append(args, clickhouse.Named("traceIDs", traceIDs))
	var rows []llmTraceRow
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "llm.QueryTraces.Aggregates", &rows, query, args...)
	return rows, err
}

func appendCursorFilter(where string, args []any, cur *traceCursor) (string, []any) {
	if cur == nil {
		return where, args
	}
	where += ` AND (timestamp, span_id) < (@curStart, @curSpanID)`
	args = append(args,
		chargs.Nanos("curStart", time.Unix(0, int64(cur.StartNs))),
		clickhouse.Named("curSpanID", cur.SpanID),
	)
	return where, args
}

// buildTraceFilters returns the predicates on LLM spans (llmWhere) and on the
// root span (rootWhere). A trace is an error when its root or any LLM span
// failed.
func buildTraceFilters(tenantID int64, req TracesQueryRequest) (llmWhere, rootWhere string, args []any) {
	args = chargs.RangeArgs(tenantID, req.StartTime, req.EndTime)
	args = filterutil.AppendIn(&llmWhere, args,
		filterutil.InClause{Column: "service", Bind: "services", Values: req.Services},
		filterutil.InClause{Column: "gen_ai_system", Bind: "vendors", Values: req.Vendors},
		filterutil.InClause{Column: "gen_ai_request_model", Bind: "models", Values: req.Models},
	)
	switch req.Status {
	case "error":
		rootWhere += ` AND (has_error OR trace_id IN ` + llmErrorTraces + `)`
	case "ok":
		rootWhere += ` AND NOT has_error AND trace_id NOT IN ` + llmErrorTraces
	}
	if req.MinDurationMs > 0 {
		rootWhere += ` AND duration_nano >= @minDurationNs`
		args = append(args, clickhouse.Named("minDurationNs", uint64(req.MinDurationMs*1e6)))
	}
	return llmWhere, rootWhere, args
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
		     AND timestamp >= @start AND timestamp < @end
		     AND trace_id = @traceID
		ORDER BY timestamp ASC, span_id ASC
		LIMIT @maxSpans`
	var rows []traceSpanRow
	args := append(chargs.RangeArgs(tenantID, startTimeMs, endTimeMs),
		clickhouse.Named("traceID", traceID),
		clickhouse.Named("ioMaxChars", uint64(traceSpanIOMaxChars)),
		clickhouse.Named("maxSpans", uint64(traceSpansMaxRows)),
	)
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "llm.TraceSpans", &rows, query,
		args...,
	)
	return rows, err
}

// TraceSpanIO fetches the untruncated prompt/completion for a single span,
// or sql.ErrNoRows.
func (r *Repository) TraceSpanIO(ctx context.Context, tenantID int64, traceID, spanID string, startTimeMs, endTimeMs int64) (spanIORow, error) {
	query := `
		SELECT gen_ai_prompt     AS prompt,
		       gen_ai_completion AS completion
		FROM optikk.spans
		PREWHERE tenant_id = @tenantID
		     AND timestamp >= @start AND timestamp < @end
		     AND trace_id = @traceID
		WHERE span_id = @spanID
		LIMIT 1`
	args := append(chargs.RangeArgs(tenantID, startTimeMs, endTimeMs),
		clickhouse.Named("traceID", traceID),
		clickhouse.Named("spanID", spanID),
	)
	var row spanIORow
	err := dbutil.QueryRowCH(dbutil.ExplorerCtx(ctx), r.db, "llm.TraceSpanIO", &row, query, args...)
	return row, err
}

func (r *Repository) ScoresForTraces(ctx context.Context, tenantID, startMs, endMs int64, traceIDs []string) ([]traceScoreRow, error) {
	if len(traceIDs) == 0 {
		return nil, nil
	}
	query := `
		SELECT trace_id, name, data_type, value, string_value, source, comment
		FROM optikk.llm_scores
		PREWHERE tenant_id = @tenantID AND timestamp >= @start AND timestamp < @end
		WHERE trace_id IN @traceIDs
		ORDER BY timestamp ASC, trace_id ASC, name ASC, span_id ASC`
	args := append(chargs.RangeArgs(tenantID, startMs, endMs), clickhouse.Named("traceIDs", traceIDs))
	var rows []traceScoreRow
	err := dbutil.SelectCH(dbutil.ExplorerCtx(ctx), r.db, "llm.ScoresForTraces", &rows, query, args...)
	return rows, err
}
