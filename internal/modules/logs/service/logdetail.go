package service

import (
	"context"

	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/shared/errorcode"

	"github.com/optikklabs/query/internal/modules/logs/models"
)

func (s *Service) GetByID(ctx context.Context, tenantID int64, id string, startMs, endMs int64) (models.Log, error) {
	row, err := s.repo.GetByID(ctx, tenantID, id, startMs, endMs)
	if err != nil {
		return models.Log{}, dbutil.NoRowsAs(err, errorcode.NotFoundError{Msg: "log not found"})
	}
	return models.MapLog(row), nil
}
