package llm

import (
	"context"
	"math"
	"sort"

	"github.com/optikklabs/query/internal/infra/cursor"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// qsAt reads one percentile; tdigest merge yields NaN when no span matched.
func qsAt(qs []float64, i int) float64 {
	if len(qs) > i && !math.IsNaN(qs[i]) {
		return qs[i]
	}
	return 0
}

func (s *Service) Apps(ctx context.Context, tenantID, startMs, endMs int64) (AppsResponse, error) {
	aggs, err := s.repo.AppAggregates(ctx, tenantID, startMs, endMs)
	if err != nil {
		return AppsResponse{}, err
	}
	models, err := s.repo.ModelBreakdown(ctx, tenantID, startMs, endMs)
	if err != nil {
		return AppsResponse{}, err
	}
	trends, err := s.repo.AppTrends(ctx, tenantID, startMs, endMs)
	if err != nil {
		return AppsResponse{}, err
	}

	primary := map[string]modelBreakdownRow{}
	for _, m := range models {
		if best, ok := primary[m.Service]; !ok || m.Spans > best.Spans {
			primary[m.Service] = m
		}
	}
	trendByService := map[string][]uint64{}
	for _, t := range trends {
		trendByService[t.Service] = append(trendByService[t.Service], t.Count)
	}

	apps := make([]App, len(aggs))
	for i, a := range aggs {
		app := App{
			Service:        a.Service,
			LLMSpans:       a.LLMSpans,
			ToolSpans:      a.ToolSpans,
			RetrievalSpans: a.RetrievalSpans,
			EmbeddingSpans: a.EmbeddingSpans,
			AgentSpans:     a.AgentSpans,
			TotalSpans:     a.TotalSpans,
			P50Ms:          qsAt(a.QS, 0),
			P95Ms:          qsAt(a.QS, 1),
			P99Ms:          qsAt(a.QS, 2),
			InputTokens:    a.InputTokens,
			OutputTokens:   a.OutputTokens,
			Cost:           a.Cost,
			Trend:          trendByService[a.Service],
		}
		if a.TotalSpans > 0 {
			app.ErrorRate = float64(a.ErrorSpans) / float64(a.TotalSpans) * 100
		}
		if best, ok := primary[a.Service]; ok {
			app.Vendor = best.Vendor
			app.PrimaryModel = best.Model
		}
		apps[i] = app
	}
	return AppsResponse{Apps: apps}, nil
}

func (s *Service) Timeseries(ctx context.Context, tenantID, startMs, endMs int64, metric string) (TimeseriesResponse, error) {
	if metric == "latency" {
		rows, err := s.repo.LatencyPercentiles(ctx, tenantID, startMs, endMs)
		if err != nil {
			return TimeseriesResponse{}, err
		}
		series := []Series{{Key: "p50"}, {Key: "p95"}, {Key: "p99"}}
		for _, r := range rows {
			t := r.BucketAt.UnixMilli()
			for i := range series {
				series[i].Points = append(series[i].Points, Point{T: t, Value: qsAt(r.QS, i)})
			}
		}
		return TimeseriesResponse{Series: series}, nil
	}

	var (
		rows []keyedBucketRow
		err  error
	)
	if metric == "spend" {
		rows, err = s.repo.SpendByVendor(ctx, tenantID, startMs, endMs)
	} else {
		rows, err = s.repo.TokensByVendor(ctx, tenantID, startMs, endMs)
	}
	if err != nil {
		return TimeseriesResponse{}, err
	}
	byKey := map[string][]Point{}
	for _, r := range rows {
		byKey[r.Key] = append(byKey[r.Key], Point{T: r.BucketAt.UnixMilli(), Value: r.Value})
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	series := make([]Series, len(keys))
	for i, k := range keys {
		series[i] = Series{Key: k, Points: byKey[k]}
	}
	return TimeseriesResponse{Series: series}, nil
}

func (s *Service) CostBreakdown(ctx context.Context, tenantID, startMs, endMs int64, groupBy string) (CostBreakdownResponse, error) {
	rows, err := s.repo.ModelBreakdown(ctx, tenantID, startMs, endMs)
	if err != nil {
		return CostBreakdownResponse{}, err
	}
	type acc struct {
		CostBreakdown
		topSpans uint64
	}
	grouped := map[string]*acc{}
	for _, r := range rows {
		key := r.Service
		switch groupBy {
		case "vendor":
			key = r.Vendor
		case "model":
			key = r.Model
		}
		g, ok := grouped[key]
		if !ok {
			g = &acc{CostBreakdown: CostBreakdown{Key: key}}
			grouped[key] = g
		}
		g.LLMSpans += r.Spans
		g.InputTokens += r.InputTokens
		g.OutputTokens += r.OutputTokens
		g.Cost += r.Cost
		if r.Spans > g.topSpans {
			g.topSpans = r.Spans
			g.Vendor = r.Vendor
		}
	}
	out := make([]CostBreakdown, 0, len(grouped))
	for _, g := range grouped {
		out = append(out, g.CostBreakdown)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Cost > out[j].Cost })
	return CostBreakdownResponse{GroupBy: groupBy, Rows: out}, nil
}

func (s *Service) QueryTraces(ctx context.Context, tenantID int64, req TracesQueryRequest) (TracesQueryResponse, error) {
	req.Limit = pickLimit(req.Limit, 50, 500)
	rows, hasMore, err := s.repo.QueryTraces(ctx, tenantID, req)
	if err != nil {
		return TracesQueryResponse{}, err
	}
	results := make([]LLMTrace, len(rows))
	for i, r := range rows {
		results[i] = LLMTrace{
			TraceID:      r.TraceID,
			StartMs:      r.StartTime.UnixMilli(),
			DurationMs:   float64(r.DurationNano) / 1e6,
			Service:      r.Service,
			Operation:    r.Operation,
			Status:       r.Status,
			HasError:     r.HasError,
			Vendor:       r.Vendor,
			Model:        r.Model,
			LLMCalls:     r.LLMCalls,
			InputTokens:  r.InputTokens,
			OutputTokens: r.OutputTokens,
			Cost:         r.Cost,
		}
	}
	info := PageInfo{HasMore: hasMore, Limit: req.Limit}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		info.NextCursor = cursor.Encode(traceCursor{StartNs: uint64(last.StartTime.UnixNano()), SpanID: last.SpanID})
	}
	return TracesQueryResponse{Results: results, PageInfo: info}, nil
}

func decodeTraceCursor(raw string) (traceCursor, bool) {
	return cursor.Decode[traceCursor](raw)
}

func pickLimit(v, def, max int) int {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}

func (s *Service) TraceDetail(ctx context.Context, tenantID int64, traceID string) (TraceDetailResponse, error) {
	rows, err := s.repo.TraceSpans(ctx, tenantID, traceID)
	if err != nil || len(rows) == 0 {
		return TraceDetailResponse{}, err
	}
	resp := TraceDetailResponse{TraceID: traceID, Spans: make([]LLMSpan, len(rows))}
	for i, r := range rows {
		cost := costOf(r.Model, r.InputTokens, r.OutputTokens)
		resp.Spans[i] = LLMSpan{
			SpanID:       r.SpanID,
			ParentSpanID: r.ParentSpanID,
			Name:         r.Name,
			Service:      r.Service,
			Operation:    r.Operation,
			Vendor:       r.Vendor,
			Model:        r.Model,
			StartMs:      r.Timestamp.UnixMilli(),
			DurationMs:   float64(r.DurationNano) / 1e6,
			HasError:     r.HasError,
			InputTokens:  r.InputTokens,
			OutputTokens: r.OutputTokens,
			Cost:         cost,
		}
		resp.InputTokens += r.InputTokens
		resp.OutputTokens += r.OutputTokens
		resp.Cost += cost
		resp.HasError = resp.HasError || r.HasError
		// root span carries the request identity and prompt/output text
		if r.ParentSpanID == "" {
			resp.Service = r.Service
			resp.StartMs = r.Timestamp.UnixMilli()
			resp.DurationMs = float64(r.DurationNano) / 1e6
			resp.Prompt = r.Prompt
			resp.Output = r.Completion
		}
	}
	return resp, nil
}
