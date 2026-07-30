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
		out[i] = models.SlowQueryPattern{
			QueryHash:      r.QueryHash,
			QueryText:      r.QueryText,
			DBSystem:       r.DBSystem,
			CollectionName: r.CollectionName,
			Namespace:      r.Namespace,
			Server:         r.Server,
			CallCount:      int64(r.CallCount),
			ErrorCount:     int64(r.ErrorCount),
		}
		if len(r.QS) >= 3 {
			p50, p95, p99 := float64(r.QS[0]), float64(r.QS[1]), float64(r.QS[2])
			out[i].P50Ms, out[i].P95Ms, out[i].P99Ms = &p50, &p95, &p99
		}
	}
	return out, nil
}
