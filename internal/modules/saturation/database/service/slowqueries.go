package service

import (
	"context"

	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	"github.com/optikklabs/query/internal/modules/saturation/database/models"
)

func (s *Service) GetSlowQueryPatterns(ctx context.Context, tenantID, startMs, endMs int64, f filter.Filters, limit int) ([]models.SlowQueryPattern, error) {
	rows, err := s.repo.GetSlowQueryPatterns(ctx, tenantID, startMs, endMs, f, limit)
	if err != nil {
		return nil, err
	}
	out := make([]models.SlowQueryPattern, len(rows))
	for i, r := range rows {
		p50, p95, p99 := float64(r.P50Ms), float64(r.P95Ms), float64(r.P99Ms)
		out[i] = models.SlowQueryPattern{
			QueryHash:      r.QueryHash,
			QueryText:      r.QueryText,
			DBSystem:       r.DBSystem,
			CollectionName: r.CollectionName,
			Namespace:      r.Namespace,
			Server:         r.Server,
			P50Ms:          &p50,
			P95Ms:          &p95,
			P99Ms:          &p99,
			CallCount:      int64(r.CallCount),
			ErrorCount:     int64(r.ErrorCount),
		}
	}
	return out, nil
}
