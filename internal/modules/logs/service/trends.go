package service

import (
	"context"
	"fmt"

	"github.com/optikklabs/query/internal/modules/logs/filter"
	"github.com/optikklabs/query/internal/modules/logs/models"
	"github.com/optikklabs/query/internal/modules/logs/repository"
)

// Summary powers POST /api/v1/logs/summary.
func (s *Service) Summary(ctx context.Context, f filter.Filters) (models.Summary, error) {
	row, err := s.repo.Summary(ctx, f)
	if err != nil {
		return models.Summary{}, fmt.Errorf("logs.Summary: %w", err)
	}
	return models.Summary{Total: row.Total, Errors: row.Errors, Warns: row.Warns}, nil
}

func (s *Service) Trend(ctx context.Context, f filter.Filters) ([]models.TrendBucket, error) {
	rows, err := s.repo.Trend(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("logs.Trend: %w", err)
	}
	return mapTrend(rows), nil
}

func mapTrend(rows []repository.TrendRow) []models.TrendBucket {
	out := make([]models.TrendBucket, len(rows))
	for i, r := range rows {
		out[i] = models.TrendBucket{
			TimeBucket: r.TimeBucket.UTC().Format("2006-01-02 15:04:05"),
			Total:      r.Total,
			Error:      r.Error,
			Warn:       r.Warn,
			Info:       r.Info,
			Debug:      r.Debug,
		}
	}
	return out
}
