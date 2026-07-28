package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/optikklabs/query/internal/infra/cursor"
	"github.com/optikklabs/query/internal/modules/logs/models"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

func (s *Service) Query(ctx context.Context, req models.QueryRequest) (models.QueryResponse, error) {
	limit := req.Limit
	cur, _ := models.DecodeCursor(req.Cursor)

	rows, err := s.repo.ListLogs(ctx, req.Filters, limit+1, cur)
	if err != nil {
		return models.QueryResponse{}, fmt.Errorf("logs.Query.list: %w", err)
	}

	rows, pageInfo := cursor.Paginate(rows, limit, func(r models.LogRow) string {
		return models.Cursor{Timestamp: r.Timestamp, LogID: r.LogID}.Encode()
	})

	return models.QueryResponse{
		Results:  models.MapLogs(rows),
		PageInfo: pageInfo,
	}, nil
}

func (s *Service) Suggest(ctx context.Context, req models.SuggestRequest, tenantID int64) (models.SuggestResponse, error) {
	limit := filterutil.PickLimit(req.Limit, 10, 50)
	var rows []models.Suggestion
	var err error
	if strings.HasPrefix(req.Field, "@") {
		rows, err = s.repo.SuggestAttribute(ctx, tenantID, req.StartTime, req.EndTime, req.Field, req.Prefix, limit)
	} else {
		rows, err = s.repo.SuggestScalar(ctx, tenantID, req.StartTime, req.EndTime, req.Field, req.Prefix, limit)
	}
	if err != nil {
		return models.SuggestResponse{}, fmt.Errorf("logs.Suggest: %w", err)
	}
	return models.SuggestResponse{Suggestions: rows}, nil
}
