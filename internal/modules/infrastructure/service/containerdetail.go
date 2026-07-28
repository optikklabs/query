package service

import (
	"context"
	"time"

	"github.com/optikklabs/query/internal/modules/infrastructure/models"
	"github.com/optikklabs/query/internal/modules/infrastructure/repository"
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesdefs"
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesgroup"
	"github.com/optikklabs/query/internal/shared/metrics"
)

func (s *Service) GetPodSeries(ctx context.Context, tenantID int64, pod, metricID string, startMs, endMs int64) ([]models.SeriesPoint, bool, error) {
	def, ok := seriesdefs.Pod.Def(metricID)
	if !ok {
		return nil, false, nil
	}
	rows, err := s.repo.QueryPodSeries(ctx, tenantID, pod, startMs, endMs, def)
	if err != nil {
		return nil, true, err
	}
	return scaleSeries(rows, def), true, nil
}

func (s *Service) GetPodOverview(ctx context.Context, tenantID int64, pod string, startMs, endMs int64) (models.PodOverview, error) {
	meta, err := s.repo.QueryPodMeta(ctx, tenantID, pod, startMs, endMs)
	if err != nil {
		return models.PodOverview{}, err
	}
	red, err := s.repo.QueryPodRED(ctx, tenantID, pod, startMs, endMs)
	if err != nil {
		return models.PodOverview{}, err
	}

	out := models.PodOverview{
		Pod:              pod,
		Host:             meta.Host,
		Containers:       seriesgroup.EmptyIfNil(meta.Containers),
		Services:         seriesgroup.EmptyIfNil(meta.Services),
		Environments:     seriesgroup.EmptyIfNil(meta.Environments),
		Namespaces:       seriesgroup.EmptyIfNil(meta.Namespaces),
		AvailableMetrics: seriesgroup.EmptyIfNil(seriesdefs.Pod.GroupsFor(meta.MetricNames)),
	}
	if !meta.LastSeen.IsZero() {
		out.LastSeen = meta.LastSeen.UTC().Format(time.RFC3339)
	}
	foldRED(red, &out)
	return out, nil
}

func foldRED(red repository.PodREDRow, out *models.PodOverview) {
	out.RequestCount = int64(red.RequestCount)
	out.ErrorCount = int64(red.ErrorCount)
	out.P95LatencyMs = float64(red.P95LatencyMs)
	if red.RequestCount == 0 {
		return
	}
	out.ErrorRate = metrics.Percentage(red.ErrorCount, red.RequestCount)
	out.AvgLatencyMs = red.DurationMsSum / float64(red.RequestCount)
}
