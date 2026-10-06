package llm

import (
	"context"
	"slices"

	"github.com/optikklabs/query/internal/infra/cursor"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/modules/llm/pricing"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/filterutil"
	"github.com/optikklabs/query/internal/shared/nullable"
)

const (
	defaultTracesLimit = 50
	maxTracesLimit     = 500
)

var (
	errTraceNotFound = errorcode.NotFoundError{Msg: "Trace not found"}
	errSpanNotFound  = errorcode.NotFoundError{Msg: "Span not found"}
	traceStatuses    = []string{"", "error", "ok"}
)

func (s *Service) QueryTraces(ctx context.Context, tenantID int64, req TracesQueryRequest) (TracesQueryResponse, error) {
	if err := filterutil.ValidateTimeRange(req.StartTime, req.EndTime); err != nil {
		return TracesQueryResponse{}, err
	}
	if !slices.Contains(traceStatuses, req.Status) {
		return TracesQueryResponse{}, errorcode.ValidationError{Msg: "status must be error or ok"}
	}
	limit, err := filterutil.Limit(req.Limit, defaultTracesLimit, maxTracesLimit)
	if err != nil {
		return TracesQueryResponse{}, err
	}
	req.Limit = limit
	cur, err := cursor.Decode[traceCursor](req.Cursor)
	if err != nil {
		return TracesQueryResponse{}, err
	}
	rows, err := s.repo.QueryTraces(ctx, tenantID, req, cur)
	if err != nil {
		return TracesQueryResponse{}, err
	}
	rows, info := cursor.Paginate(rows, req.Limit, func(r llmTraceRow) string {
		return cursor.Encode(traceCursor{StartNs: uint64(r.StartTime.UnixNano()), SpanID: r.SpanID})
	})
	results := make([]LLMTrace, len(rows))
	traceIDs := make([]string, len(rows))
	for i, r := range rows {
		traceIDs[i] = r.TraceID
		results[i] = LLMTrace{
			TraceID:       r.TraceID,
			StartMs:       r.StartTime.UnixMilli(),
			DurationMs:    float64(r.DurationNano) / 1e6,
			Service:       r.Service,
			Operation:     r.Operation,
			Status:        r.Status,
			HasError:      r.HasError,
			Level:         levelOf(r.HasError),
			Vendor:        r.Vendor,
			Model:         r.Model,
			UserID:        r.UserID,
			SessionID:     r.SessionID,
			Tags:          nullable.OrEmpty(r.Tags),
			LLMCalls:      r.LLMCalls,
			PromptPreview: r.PromptPreview,
			InputTokens:   r.InputTokens,
			OutputTokens:  r.OutputTokens,
			Cost:          r.Cost,
		}
	}
	scores, err := s.repo.ScoresForTraces(ctx, tenantID, req.StartTime, req.EndTime, traceIDs)
	if err != nil {
		return TracesQueryResponse{}, err
	}
	byTrace := groupScores(scores)
	for i := range results {
		results[i].Scores = nullable.OrEmpty(byTrace[results[i].TraceID])
	}
	return TracesQueryResponse{Results: results, PageInfo: info}, nil
}

func levelOf(hasError bool) string {
	if hasError {
		return "ERROR"
	}
	return "DEFAULT"
}

func groupScores(rows []traceScoreRow) map[string][]TraceScore {
	out := make(map[string][]TraceScore)
	for _, r := range rows {
		out[r.TraceID] = append(out[r.TraceID], TraceScore{
			Name:     r.Name,
			DataType: r.DataType,
			Value:    r.Value,
			String:   r.String,
			Source:   r.Source,
			Comment:  r.Comment,
		})
	}
	return out
}

func (s *Service) TraceDetail(ctx context.Context, tenantID int64, traceID string, startTimeMs, endTimeMs int64) (TraceDetailResponse, error) {
	rows, err := s.repo.TraceSpans(ctx, tenantID, traceID, startTimeMs, endTimeMs)
	if err != nil {
		return TraceDetailResponse{}, err
	}
	if len(rows) == 0 {
		return TraceDetailResponse{}, errTraceNotFound
	}
	resp := buildTraceDetail(traceID, rows)
	scores, err := s.repo.ScoresForTraces(ctx, tenantID, startTimeMs, endTimeMs, []string{traceID})
	if err != nil {
		return TraceDetailResponse{}, err
	}
	resp.Scores = nullable.OrEmpty(groupScores(scores)[traceID])
	fillTraceIO(&resp, rows)
	return resp, nil
}

func buildTraceDetail(traceID string, rows []traceSpanRow) TraceDetailResponse {
	resp := TraceDetailResponse{TraceID: traceID, Spans: make([]LLMSpan, len(rows))}
	for i, r := range rows {
		span := mapLLMSpan(r)
		resp.Spans[i] = span
		resp.InputTokens += r.InputTokens
		resp.OutputTokens += r.OutputTokens
		resp.Cost += span.Cost
		resp.HasError = resp.HasError || r.HasError
		if r.ParentSpanID == "" {
			applyTraceRoot(&resp, r)
		}
	}
	return resp
}

func mapLLMSpan(r traceSpanRow) LLMSpan {
	return LLMSpan{
		SpanID: r.SpanID, ParentSpanID: r.ParentSpanID, Name: r.Name,
		Service: r.Service, Operation: r.Operation, Kind: r.Kind, Vendor: r.Vendor,
		Model: r.Model, ResponseModel: r.ResponseModel, StartMs: r.Timestamp.UnixMilli(),
		DurationMs: float64(r.DurationNano) / 1e6, HasError: r.HasError,
		InputTokens: r.InputTokens, OutputTokens: r.OutputTokens,
		Cost:   pricing.CostOf(r.Model, r.InputTokens, r.OutputTokens),
		Prompt: r.Prompt, Completion: r.Completion,
		PromptTruncated: r.PromptTruncated != 0, CompletionTruncated: r.CompletionTruncated != 0,
	}
}

func applyTraceRoot(resp *TraceDetailResponse, r traceSpanRow) {
	resp.Name, resp.Service, resp.Environment = r.Name, r.Service, r.Environment
	resp.UserID, resp.SessionID, resp.Release = r.UserID, r.SessionID, r.Release
	resp.StartMs, resp.DurationMs = r.Timestamp.UnixMilli(), float64(r.DurationNano)/1e6
	resp.Prompt, resp.Output = r.Prompt, r.Completion
}

func fillTraceIO(resp *TraceDetailResponse, rows []traceSpanRow) {
	for _, r := range rows {
		if resp.Prompt != "" {
			break
		}
		resp.Prompt = r.Prompt
	}
	if resp.Output == "" {
		for i := len(rows) - 1; i >= 0 && resp.Output == ""; i-- {
			resp.Output = rows[i].Completion
		}
	}
}

// SpanIO returns the untruncated prompt/completion for a single span.
func (s *Service) SpanIO(ctx context.Context, tenantID int64, traceID, spanID string, startTimeMs, endTimeMs int64) (SpanIOResponse, error) {
	row, err := s.repo.TraceSpanIO(ctx, tenantID, traceID, spanID, startTimeMs, endTimeMs)
	if err != nil {
		return SpanIOResponse{}, dbutil.NoRowsAs(err, errSpanNotFound)
	}
	return SpanIOResponse{
		TraceID:    traceID,
		SpanID:     spanID,
		Prompt:     row.Prompt,
		Completion: row.Completion,
	}, nil
}
