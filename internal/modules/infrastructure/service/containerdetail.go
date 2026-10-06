package service

import (
	"context"

	"github.com/optikklabs/query/internal/shared/nullable"

	"github.com/optikklabs/query/internal/modules/infrastructure/models"
	"github.com/optikklabs/query/internal/modules/infrastructure/repository"
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesdefs"
	"github.com/optikklabs/query/internal/shared/metrics"
)

func (s *Service) GetPodSeries(ctx context.Context, tenantID int64, pod, metricID string, startMs, endMs int64) ([]models.SeriesPoint, error) {
	def, ok := seriesdefs.Pod.Def(metricID)
	if !ok {
		return nil, errUnknownMetricGroup
	}
	rows, err := s.repo.QueryPodSeries(ctx, tenantID, pod, startMs, endMs, def)
	if err != nil {
		return nil, err
	}
	return scaleSeries(rows, def), nil
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
		LastSeen:         meta.LastSeen,
		Containers:       nullable.OrEmpty(meta.Containers),
		Services:         nullable.OrEmpty(meta.Services),
		Environments:     nullable.OrEmpty(meta.Environments),
		Namespaces:       nullable.OrEmpty(meta.Namespaces),
		AvailableMetrics: nullable.OrEmpty(seriesdefs.Pod.GroupsFor(meta.MetricNames)),
	}
	foldRED(red, &out)
	return out, nil
}

func foldRED(red repository.PodREDRow, out *models.PodOverview) {
	out.RequestCount = int64(red.RequestCount)
	out.ErrorCount = int64(red.ErrorCount)
	if red.RequestCount == 0 {
		return
	}
	out.ErrorRate = new(metrics.Percentage(red.ErrorCount, red.RequestCount))
	out.AvgLatencyMs = new(red.DurationMsSum / float64(red.RequestCount))
	out.P95LatencyMs = new(float64(red.P95LatencyMs))
}
