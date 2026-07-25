package containerdetail

import (
	"context"
	"time"

	"github.com/optikklabs/query/internal/modules/infrastructure/seriesgroup"
	"github.com/optikklabs/query/internal/shared/metrics"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetSeries(ctx context.Context, tenantID int64, pod, metricID string, startMs, endMs int64) ([]SeriesPoint, bool, error) {
	def, ok := catalog.Def(metricID)
	if !ok {
		return nil, false, nil
	}
	rows, err := s.repo.QuerySeries(ctx, tenantID, pod, startMs, endMs, def)
	if err != nil {
		return nil, true, err
	}
	if def.Scale != 1 {
		for i := range rows {
			rows[i].Value *= def.Scale
		}
	}
	if rows == nil {
		rows = []SeriesPoint{}
	}
	return rows, true, nil
}

func (s *Service) GetOverview(ctx context.Context, tenantID int64, pod string, startMs, endMs int64) (PodOverview, error) {
	meta, err := s.repo.QueryPodMeta(ctx, tenantID, pod, startMs, endMs)
	if err != nil {
		return PodOverview{}, err
	}
	red, err := s.repo.QueryPodRED(ctx, tenantID, pod, startMs, endMs)
	if err != nil {
		return PodOverview{}, err
	}

	out := PodOverview{
		Pod:              pod,
		Host:             meta.Host,
		Containers:       seriesgroup.EmptyIfNil(meta.Containers),
		Services:         seriesgroup.EmptyIfNil(meta.Services),
		Environments:     seriesgroup.EmptyIfNil(meta.Environments),
		Namespaces:       seriesgroup.EmptyIfNil(meta.Namespaces),
		AvailableMetrics: seriesgroup.EmptyIfNil(catalog.GroupsFor(meta.MetricNames)),
	}
	if !meta.LastSeen.IsZero() {
		out.LastSeen = meta.LastSeen.UTC().Format(time.RFC3339)
	}
	foldRED(red, &out)
	return out, nil
}

// foldRED derives rate/latency KPIs from raw span-metric aggregates.
func foldRED(red podREDRow, out *PodOverview) {
	out.RequestCount = int64(red.RequestCount)
	out.ErrorCount = int64(red.ErrorCount)
	out.P95LatencyMs = float64(red.P95LatencyMs)
	if red.RequestCount == 0 {
		return
	}
	out.ErrorRate = metrics.Percentage(red.ErrorCount, red.RequestCount)
	out.AvgLatencyMs = red.DurationMsSum / float64(red.RequestCount)
}
