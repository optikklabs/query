package service

import (
	"context"

	"github.com/optikklabs/query/internal/modules/logs/models"
)

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
