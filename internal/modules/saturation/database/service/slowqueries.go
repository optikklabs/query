package service

import (
	"context"

	"github.com/optikklabs/query/internal/infra/cursor"
	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	"github.com/optikklabs/query/internal/modules/saturation/database/models"
	"github.com/optikklabs/query/internal/modules/saturation/database/repository"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

func (s *Service) GetSlowQueryPatterns(ctx context.Context, tenantID, startMs, endMs int64, f filter.Filters, limit int) ([]models.SlowQueryPattern, error) {
	rows, err := s.repo.GetSlowQueryPatterns(ctx, tenantID, startMs, endMs, f, limit)
	if err != nil {
		return nil, err
	}
	return toSlowQueryPatterns(rows), nil
}

func (s *Service) QueryPatterns(
	ctx context.Context,
	tenantID, startMs, endMs int64,
	f filter.ExplorerFilters,
	limit int,
	rawCursor string,
) (models.QueryPatternsPage, error) {
	limit = filterutil.PickLimit(limit, repository.DefaultPatternLimit, 200)
	cur, _ := cursor.Decode[repository.QueryPatternsCursor](rawCursor)
	rows, err := s.repo.QueryPatterns(ctx, tenantID, startMs, endMs, f, limit+1, cur)
	if err != nil {
		return models.QueryPatternsPage{}, err
	}
	rows, pageInfo := cursor.Paginate(rows, limit, func(row repository.PatternRaw) string {
		return cursor.Encode(repository.QueryPatternsCursor{
			CallCount:      row.CallCount,
			QueryHash:      row.QueryHash,
			DBSystem:       row.DBSystem,
			CollectionName: row.CollectionName,
		})
	})
	return models.QueryPatternsPage{
		Results:  toSlowQueryPatterns(rows),
		PageInfo: pageInfo,
	}, nil
}

func toSlowQueryPatterns(rows []repository.PatternRaw) []models.SlowQueryPattern {
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
	return out
}
