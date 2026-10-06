package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/optikklabs/query/internal/infra/cursor"
	"github.com/optikklabs/query/internal/modules/logs/models"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

const (
	defaultQueryLimit   = 100
	maxQueryLimit       = 5000
	defaultSuggestLimit = 10
	maxSuggestLimit     = 50
)

func (s *Service) Query(ctx context.Context, req models.QueryRequest) (models.QueryResponse, error) {
	limit, err := filterutil.Limit(req.Limit, defaultQueryLimit, maxQueryLimit)
	if err != nil {
		return models.QueryResponse{}, err
	}
	cur, err := cursor.Decode[models.Cursor](req.Cursor)
	if err != nil {
		return models.QueryResponse{}, err
	}
	rows, err := s.repo.ListLogs(ctx, req.Filters, limit+1, cur)
	if err != nil {
		return models.QueryResponse{}, fmt.Errorf("logs.Query.list: %w", err)
	}

	rows, pageInfo := cursor.Paginate(rows, limit, func(r models.LogRow) string {
		return cursor.Encode(models.Cursor{Timestamp: r.Timestamp, LogID: r.LogID})
	})

	return models.QueryResponse{
		Results:  models.MapLogs(rows),
		PageInfo: pageInfo,
	}, nil
}

func (s *Service) Suggest(ctx context.Context, req models.SuggestRequest, tenantID int64) (models.SuggestResponse, error) {
	limit, err := filterutil.Limit(req.Limit, defaultSuggestLimit, maxSuggestLimit)
	if err != nil {
		return models.SuggestResponse{}, err
	}
	var rows []models.Suggestion
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
