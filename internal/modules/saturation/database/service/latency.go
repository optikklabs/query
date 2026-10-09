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
		out[i] = models.LatencyTimeSeries{
			TimeBucketMs: r.BucketAt.UnixMilli(),
			GroupBy:      r.GroupBy,
		}
		if len(r.QS) >= 3 {
			out[i].P50Ms, out[i].P95Ms, out[i].P99Ms = &r.QS[0], &r.QS[1], &r.QS[2]
		}
	}
	return out
}
