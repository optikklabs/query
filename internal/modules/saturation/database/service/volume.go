package service

import (
	"context"

	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	"github.com/optikklabs/query/internal/modules/saturation/database/models"
	"github.com/optikklabs/query/internal/modules/saturation/database/repository"
)

func (s *Service) GetOpsBySystem(ctx context.Context, tenantID, startMs, endMs int64, f filter.Filters) ([]models.OpsTimeSeries, error) {
	rows, err := s.repo.GetOpsBySystem(ctx, tenantID, startMs, endMs, f)
	return mapOpsRate(rows), err
}

func mapOpsRate(rows []repository.OpsRaw) []models.OpsTimeSeries {
	if rows == nil {
		return nil
	}
	out := make([]models.OpsTimeSeries, len(rows))
	for i, r := range rows {
		rate := r.OpsPerSec
		out[i] = models.OpsTimeSeries{TimeBucketMs: r.TimeBucket.UnixMilli(), GroupBy: r.GroupBy, OpsPerSec: &rate}
	}
	return out
}
