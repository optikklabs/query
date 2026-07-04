package trace_logs

import (
	"context"

	"github.com/optikklabs/query/internal/modules/logs/shared/models"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// GetByTraceID resolves all logs for a (tenant_id, trace_id) pair.
func (s *Service) GetByTraceID(ctx context.Context, tenantID int64, traceID string, limit int) ([]models.Log, error) {
	rows, err := s.repo.FetchLogsByTrace(ctx, tenantID, traceID, limit)
	if err != nil {
		return nil, err
	}
	return models.MapLogs(rows), nil
}
