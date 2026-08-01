package service

import (
	"context"
	"time"

	"github.com/optikklabs/query/internal/infra/cursor"
	"github.com/optikklabs/query/internal/modules/services/errors/models"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

const (
	defaultGroupsLimit = 25
	maxGroupsLimit     = 200
	// An issue counts as "new" when its first occurrence in the range is
	// within this window — the KPI strip's "first seen < 24h".
	newIssueWindow = 24 * time.Hour
)

func (s *Service) QueryErrorGroups(ctx context.Context, req models.GroupsRequest) (models.GroupsResponse, error) {
	limit := filterutil.PickLimit(req.Limit, defaultGroupsLimit, maxGroupsLimit)
	req.Limit = limit + 1

	raw, err := s.repo.ExplorerGroupRows(ctx, req)
	if err != nil {
		return models.GroupsResponse{}, err
	}
	raw, pageInfo := cursor.Paginate(raw, limit, func(r models.RawErrorGroupRow) string {
		return cursor.Encode(models.ErrorGroupsCursor{ErrorCount: r.ErrorCount, GroupID: r.GroupID})
	})

	results := make([]models.ErrorGroup, len(raw))
	for i, row := range raw {
		results[i] = toErrorGroup(row)
	}
	return models.GroupsResponse{Results: results, PageInfo: pageInfo}, nil
}

func (s *Service) QueryErrorFacets(ctx context.Context, req models.FacetsRequest) (models.Facets, error) {
	rows, err := s.repo.ExplorerFacetRows(ctx, req)
	if err != nil {
		return models.Facets{}, err
	}

	var facets models.Facets
	for _, row := range rows {
		bucket := models.FacetBucket{Value: row.Value, Count: int64(row.Count)}
		switch row.Dim {
		case "service":
			facets.Service = append(facets.Service, bucket)
		case "operation":
			facets.Operation = append(facets.Operation, bucket)
		case "httpStatus":
			facets.HTTPStatus = append(facets.HTTPStatus, bucket)
		case "exceptionType":
			facets.ExceptionType = append(facets.ExceptionType, bucket)
		}
	}
	return facets, nil
}

func (s *Service) QueryErrorOverview(ctx context.Context, req models.OverviewRequest) (models.OverviewResponse, error) {
	newSinceMs := time.Now().Add(-newIssueWindow).UnixMilli()
	summary, err := s.repo.ExplorerSummaryRow(ctx, req, newSinceMs)
	if err != nil {
		return models.OverviewResponse{}, err
	}
	trendRows, err := s.repo.ExplorerTrendRows(ctx, req)
	if err != nil {
		return models.OverviewResponse{}, err
	}

	trend := make([]models.TrendBucket, len(trendRows))
	for i, row := range trendRows {
		trend[i] = models.TrendBucket{TimeBucketMs: row.TimeBucket.UnixMilli(), Errors: int64(row.Errors)}
	}

	return models.OverviewResponse{
		Summary: models.Summary{
			TotalErrors:      int64(summary.TotalErrors),
			ActiveIssues:     int64(summary.ActiveIssues),
			NewIssues:        int64(summary.NewIssues),
			ServicesAffected: int64(summary.ServicesAffected),
		},
		Trend: trend,
	}, nil
}

func toErrorGroup(row models.RawErrorGroupRow) models.ErrorGroup {
	return models.ErrorGroup{
		GroupID:         row.GroupID,
		ServiceName:     row.ServiceName,
		OperationName:   row.OperationName,
		StatusMessage:   row.StatusMessage,
		HTTPStatusCode:  httpBucketToCode(row.HTTPStatusBucket),
		ErrorCount:      int64(row.ErrorCount),
		LastOccurrence:  row.LastOccurrence,
		FirstOccurrence: row.FirstOccurrence,
		SampleTraceID:   row.SampleTraceID,
	}
}
