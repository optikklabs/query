package service

import (
	"context"

	"github.com/optikklabs/query/internal/infra/cursor"
	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	"github.com/optikklabs/query/internal/modules/saturation/database/models"
	"github.com/optikklabs/query/internal/modules/saturation/database/repository"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

const maxPatternLimit = 200

func (s *Service) QueryPatterns(
	ctx context.Context,
	tenantID, startMs, endMs int64,
	f filter.ExplorerFilters,
	limit int,
	rawCursor string,
) (models.QueryPatternsPage, error) {
	limit, err := filterutil.Limit(limit, repository.DefaultPatternLimit, maxPatternLimit)
	if err != nil {
		return models.QueryPatternsPage{}, err
	}
	cur, err := cursor.Decode[repository.QueryPatternsCursor](rawCursor)
	if err != nil {
		return models.QueryPatternsPage{}, err
	}
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
			P50Ms:          r.QS[0],
			P95Ms:          r.QS[1],
			P99Ms:          r.QS[2],
			CallCount:      int64(r.CallCount),
			ErrorCount:     int64(r.ErrorCount),
		}
	}
	return out
}
