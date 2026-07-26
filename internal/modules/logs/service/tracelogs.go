package service

import (
	"context"

	"github.com/optikklabs/query/internal/modules/logs/models"
)

// GetByTraceID fetches every log correlated with a trace, by trace id alone.
func (s *Service) GetByTraceID(ctx context.Context, tenantID int64, traceID string, limit int) ([]models.Log, error) {
	rows, err := s.repo.FetchLogsByTrace(ctx, tenantID, traceID, limit)
	if err != nil {
		return nil, err
	}
	return models.MapLogs(rows), nil
}
