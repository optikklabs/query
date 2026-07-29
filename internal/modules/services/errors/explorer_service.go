package errors

import (
	"context"
	"time"

	"github.com/optikklabs/query/internal/infra/cursor"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

const (
	defaultGroupsLimit = 25
	maxGroupsLimit     = 200
	// An issue counts as "new" when its first occurrence in the range is
	// within this window — the KPI strip's "first seen < 24h".
	newIssueWindow = 24 * time.Hour
)

func decodeGroupsCursor(raw string) (ErrorGroupsCursor, bool) {
	return cursor.Decode[ErrorGroupsCursor](raw)
}

func millisToTime(ms int64) time.Time { return time.UnixMilli(ms) }

func (s *Service) QueryErrorGroups(ctx context.Context, req GroupsRequest) (GroupsResponse, error) {
	limit := filterutil.PickLimit(req.Limit, defaultGroupsLimit, maxGroupsLimit)
	req.Limit = limit + 1

	raw, err := s.repo.ExplorerGroupRows(ctx, req)
	if err != nil {
		return GroupsResponse{}, err
	}
	raw, pageInfo := cursor.Paginate(raw, limit, func(r rawErrorGroupRow) string {
		return cursor.Encode(ErrorGroupsCursor{ErrorCount: r.ErrorCount, GroupID: r.GroupID})
	})

	results := make([]ErrorGroup, len(raw))
	for i, row := range raw {
		results[i] = toErrorGroup(row)
	}
	return GroupsResponse{Results: results, PageInfo: pageInfo}, nil
}

func (s *Service) QueryErrorFacets(ctx context.Context, req FacetsRequest) (Facets, error) {
	rows, err := s.repo.ExplorerFacetRows(ctx, req)
	if err != nil {
		return Facets{}, err
	}

	var facets Facets
	for _, row := range rows {
		bucket := FacetBucket{Value: row.Value, Count: int64(row.Count)}
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

func (s *Service) QueryErrorOverview(ctx context.Context, req OverviewRequest) (OverviewResponse, error) {
	newSinceMs := time.Now().Add(-newIssueWindow).UnixMilli()
	summary, err := s.repo.ExplorerSummaryRow(ctx, req, newSinceMs)
	if err != nil {
		return OverviewResponse{}, err
	}
	trendRows, err := s.repo.ExplorerTrendRows(ctx, req)
	if err != nil {
		return OverviewResponse{}, err
	}

	trend := make([]TrendBucket, len(trendRows))
	for i, row := range trendRows {
		trend[i] = TrendBucket{TimeBucket: row.TimeBucket, Errors: int64(row.Errors)}
	}
	return OverviewResponse{
		Summary: Summary{
			TotalErrors:      int64(summary.TotalErrors),
			ActiveIssues:     int64(summary.ActiveIssues),
			NewIssues:        int64(summary.NewIssues),
			ServicesAffected: int64(summary.ServicesAffected),
		},
		Trend: trend,
	}, nil
}

func toErrorGroup(row rawErrorGroupRow) ErrorGroup {
	return ErrorGroup{
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
