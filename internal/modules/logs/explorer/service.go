package explorer

import (
	"context"
	"fmt"
	"strings"

	"github.com/optikklabs/query/internal/modules/logs/shared/models"
)

const (
	defaultSuggestLimit = 10
	maxSuggestLimit     = 50
)

// Service orchestrates POST /api/v1/logs/query. It owns the list path.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Query(ctx context.Context, req QueryRequest) (QueryResponse, error) {
	limit := req.Limit
	cur, _ := models.DecodeCursor(req.Cursor)

	rows, hasMore, err := s.repo.getLogs(ctx, req.Filters, limit, cur)
	if err != nil {
		return QueryResponse{}, fmt.Errorf("logs.Query.list: %w", err)
	}

	return QueryResponse{
		Results:  models.MapLogs(rows),
		PageInfo: buildPageInfo(rows, hasMore, limit),
	}, nil
}

// Suggest returns value suggestions for a scalar field or @attribute key,
// mirroring the traces suggest behavior.
func (s *Service) Suggest(ctx context.Context, req SuggestRequest, tenantID int64) (SuggestResponse, error) {
	limit := pickSuggestLimit(req.Limit)
	var rows []Suggestion
	var err error
	if strings.HasPrefix(req.Field, "@") {
		rows, err = s.repo.SuggestAttribute(ctx, tenantID, req.StartTime, req.EndTime, req.Field, req.Prefix, limit)
	} else {
		rows, err = s.repo.SuggestScalar(ctx, tenantID, req.StartTime, req.EndTime, req.Field, req.Prefix, limit)
	}
	if err != nil {
		return SuggestResponse{}, fmt.Errorf("logs.Suggest: %w", err)
	}
	return SuggestResponse{Suggestions: rows}, nil
}

func pickSuggestLimit(v int) int {
	if v <= 0 {
		return defaultSuggestLimit
	}
	if v > maxSuggestLimit {
		return maxSuggestLimit
	}
	return v
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
