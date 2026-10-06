package service

import (
	"context"

	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	"github.com/optikklabs/query/internal/modules/saturation/database/models"
	"github.com/optikklabs/query/internal/modules/saturation/database/repository"
)

func (s *Service) GetLatencyBySystem(ctx context.Context, tenantID, startMs, endMs int64, f filter.Filters) ([]models.LatencyTimeSeries, error) {
	rows, err := s.repo.GetLatencyBySystem(ctx, tenantID, startMs, endMs, f)
	if err != nil {
		return nil, err
	}
	return foldLatency(rows), nil
}

func foldLatency(rows []repository.LatencyRaw) []models.LatencyTimeSeries {
	out := make([]models.LatencyTimeSeries, len(rows))
	for i, r := range rows {
		p50, p95, p99 := float64(r.P50Ms), float64(r.P95Ms), float64(r.P99Ms)
		out[i] = models.LatencyTimeSeries{
			TimeBucketMs: r.BucketAt.UnixMilli(),
			GroupBy:      r.GroupBy,
			P50Ms:        &p50,
			P95Ms:        &p95,
			P99Ms:        &p99,
		}
	}
	return out
}
