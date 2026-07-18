package trace_logs

import (
	"context"

	"github.com/optikklabs/query/internal/modules/logs/shared/models"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// GetByTraceID fetches logs for a trace inside the caller-supplied range.
func (s *Service) GetByTraceID(ctx context.Context, tenantID int64, traceID string, limit int, startTimeMs, endTimeMs int64) ([]models.Log, error) {
	rows, err := s.repo.FetchLogsByTrace(ctx, tenantID, traceID, limit, startTimeMs, endTimeMs)
	if err != nil {
		return nil, err
	}
	return models.MapLogs(rows), nil
}
