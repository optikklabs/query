package service

import (
	"context"

	"github.com/optikklabs/query/internal/modules/logs/models"
)

// GetByID reads a single log by its deep-link id (a 16-char hex hash stored
// directly as the row's log_id column). Returns nil on not-found.
func (s *Service) GetByID(ctx context.Context, tenantID int64, id string, startMs, endMs int64) (*models.Log, error) {
	row, err := s.repo.GetByID(ctx, tenantID, id, startMs, endMs)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	out := models.MapLog(*row)
	return &out, nil
}
