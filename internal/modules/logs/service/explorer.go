package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/optikklabs/query/internal/modules/logs/models"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

// Query powers POST /api/v1/logs/query — the keyset-paginated log list.
func (s *Service) Query(ctx context.Context, req models.QueryRequest) (models.QueryResponse, error) {
	limit := req.Limit
	cur, _ := models.DecodeCursor(req.Cursor)

	rows, hasMore, err := s.repo.ListLogs(ctx, req.Filters, limit, cur)
	if err != nil {
		return models.QueryResponse{}, fmt.Errorf("logs.Query.list: %w", err)
	}

	return models.QueryResponse{
		Results:  models.MapLogs(rows),
		PageInfo: buildPageInfo(rows, hasMore, limit),
	}, nil
}

// Suggest returns value suggestions for a scalar field or @attribute key,
// mirroring the traces suggest behavior.
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

func buildPageInfo(rows []models.LogRow, hasMore bool, limit int) models.PageInfo {
	info := models.PageInfo{HasMore: hasMore, Limit: limit}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		info.NextCursor = models.Cursor{
			Timestamp: last.Timestamp,
			LogID:     last.LogID,
		}.Encode()
	}
	return info
}
