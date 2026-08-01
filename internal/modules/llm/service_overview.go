package llm

import (
	"context"
	"sort"

	"github.com/optikklabs/query/internal/shared/metrics"
	"golang.org/x/sync/errgroup"
)

func (s *Service) Apps(ctx context.Context, tenantID, startMs, endMs int64) (AppsResponse, error) {
	var (
		aggs   []appAggRow
		models []modelBreakdownRow
		trends []trendRow
	)
	g, groupCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		aggs, err = s.repo.AppAggregates(groupCtx, tenantID, startMs, endMs)
		return wrapLLMError("app aggregates", err)
	})
	g.Go(func() error {
		var err error
		models, err = s.repo.ModelBreakdown(groupCtx, tenantID, startMs, endMs)
		return wrapLLMError("model breakdown", err)
	})
	g.Go(func() error {
		var err error
		trends, err = s.repo.AppTrends(groupCtx, tenantID, startMs, endMs)
		return wrapLLMError("app trends", err)
	})
	if err := g.Wait(); err != nil {
		return AppsResponse{}, err
	}
	return buildAppsResponse(aggs, models, trends), nil
}

func buildAppsResponse(aggs []appAggRow, models []modelBreakdownRow, trends []trendRow) AppsResponse {
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
		app.Kind = deriveKind(a)
		app.ErrorRate = metrics.Percentage(a.ErrorSpans, a.TotalSpans)
		if best, ok := primary[a.Service]; ok {
			app.Vendor = best.Vendor
			app.PrimaryModel = best.Model
		}
		apps[i] = app
	}
	return AppsResponse{Apps: apps}
}

func deriveKind(a appAggRow) string {
	switch {
	case a.AgentSpans > 0:
		return "agent"
	case a.RetrievalSpans > 0:
		return "rag"
	default:
		return "workflow"
	}
}

func (s *Service) Models(ctx context.Context, tenantID, startMs, endMs int64) (ModelsResponse, error) {
	rows, err := s.repo.ModelUsage(ctx, tenantID, startMs, endMs)
	if err != nil {
		return ModelsResponse{}, err
	}
	models := make([]ModelUsage, len(rows))
	for i, r := range rows {
		models[i] = ModelUsage{
			Model:        r.Model,
			Vendor:       r.Vendor,
			Traces:       r.Traces,
			InputTokens:  r.InputTokens,
			OutputTokens: r.OutputTokens,
			P50Ms:        qsAt(r.QS, 0),
			P95Ms:        qsAt(r.QS, 1),
			Cost:         r.Cost,
		}
	}
	return ModelsResponse{Models: models}, nil
}

func (s *Service) Overview(ctx context.Context, tenantID, startMs, endMs int64) (OverviewResponse, error) {
	var (
		windows []overviewWindowRow
		counts  []traceCountRow
		series  []overviewSeriesRow
	)
	g, groupCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		windows, err = s.repo.OverviewWindows(groupCtx, tenantID, startMs, endMs)
		return wrapLLMError("overview windows", err)
	})
	g.Go(func() error {
		var err error
		counts, err = s.repo.TraceCounts(groupCtx, tenantID, startMs, endMs)
		return wrapLLMError("trace counts", err)
	})
	g.Go(func() error {
		var err error
		series, err = s.repo.OverviewSeries(groupCtx, tenantID, startMs, endMs)
		return wrapLLMError("overview series", err)
	})
	if err := g.Wait(); err != nil {
		return OverviewResponse{}, err
	}

	var resp OverviewResponse
	for _, w := range windows {
		win := OverviewWindow{
			LLMSpans:     w.LLMSpans,
			ToolSpans:    w.ToolSpans,
			TotalSpans:   w.TotalSpans,
			InputTokens:  w.InputTokens,
			OutputTokens: w.OutputTokens,
			ErrorRate:    metrics.Percentage(w.ErrorSpans, w.TotalSpans),
			P50Ms:        qsAt(w.QS, 0),
			P95Ms:        qsAt(w.QS, 1),
			P99Ms:        qsAt(w.QS, 2),
			Cost:         w.Cost,
		}
		if w.IsCurrent == 1 {
			resp.Current = win
		} else {
			resp.Previous = win
		}
	}
	for _, c := range counts {
		if c.IsCurrent == 1 {
			resp.Current.Traces = c.Traces
		} else {
			resp.Previous.Traces = c.Traces
		}
	}
	resp.Series = transposeOverviewSeries(series)
	return resp, nil
}

func transposeOverviewSeries(rows []overviewSeriesRow) OverviewSeries {
	out := OverviewSeries{
		Timestamps: make([]int64, len(rows)),
		LLMSpans:   make([]uint64, len(rows)),
		ToolSpans:  make([]uint64, len(rows)),
		ErrorRate:  make([]float64, len(rows)),
		P95Ms:      make([]float64, len(rows)),
		Cost:       make([]float64, len(rows)),
	}
	for i, r := range rows {
		out.Timestamps[i] = r.BucketAt.UnixMilli()
		out.LLMSpans[i] = r.LLMSpans
		out.ToolSpans[i] = r.ToolSpans
		out.ErrorRate[i] = metrics.Percentage(r.ErrorSpans, r.TotalSpans)
		out.P95Ms[i] = qsAt(r.QS, 1)
		out.Cost[i] = r.Cost
	}
	return out
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
	rows, err := s.repo.CostBreakdownByKey(ctx, tenantID, startMs, endMs, groupBy)
	if err != nil {
		return CostBreakdownResponse{}, err
	}
	out := make([]CostBreakdown, len(rows))
	for i, r := range rows {
		out[i] = CostBreakdown{
			Key:          r.Key,
			Vendor:       r.TopVendor,
			LLMSpans:     r.Spans,
			InputTokens:  r.InputTokens,
			OutputTokens: r.OutputTokens,
			Cost:         r.Cost,
		}
	}
	return CostBreakdownResponse{GroupBy: groupBy, Rows: out}, nil
}
