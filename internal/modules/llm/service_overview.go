package llm

import (
	"context"

	"golang.org/x/sync/errgroup"

	"github.com/optikklabs/query/internal/shared/metrics"
)

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
