package redfleet

import (
	"context"

	"github.com/optikklabs/query/internal/infra/cursor"
	"github.com/optikklabs/query/internal/shared/httputil"
)

func (s *Service) GetTopEndpointsCombined(
	ctx context.Context, f REDFilters, limit int, cursorIn TopEndpointsCursor,
) (PaginatedEndpoints, error) {
	rows, err := s.repo.GetTopEndpointsCombined(ctx, f, limit+1, cursorIn)
	if err != nil {
		return PaginatedEndpoints{}, err
	}

	rows, pageInfo := cursor.Paginate(rows, limit, func(r topEndpointRow) string {
		return cursor.Encode(TopEndpointsCursor{TotalCount: r.TotalCount, OperationName: r.OperationName})
	})

	durationSec := float64(f.EndMs-f.StartMs) / 1000.0
	if durationSec <= 0 {
		durationSec = 1
	}

	results := make([]TopEndpoint, len(rows))
	for i, row := range rows {
		results[i] = toTopEndpoint(row, durationSec)
	}

	return PaginatedEndpoints{Results: results, PageInfo: pageInfo}, nil
}

func (s *Service) GetTopDBQueries(
	ctx context.Context, f REDFilters, limit int, cursorIn TopEndpointsCursor,
) (PaginatedDBQueries, error) {
	rows, err := s.repo.GetTopDBQueriesCombined(ctx, f, limit+1, cursorIn)
	if err != nil {
		return PaginatedDBQueries{}, err
	}

	rows, pageInfo := cursor.Paginate(rows, limit, func(r topDBQueryRow) string {
		return cursor.Encode(TopEndpointsCursor{TotalCount: r.TotalCount, OperationName: r.OperationName})
	})

	durationSec := float64(f.EndMs-f.StartMs) / 1000.0
	if durationSec <= 0 {
		durationSec = 1
	}

	results := make([]TopDBQuery, len(rows))
	for i, row := range rows {
		results[i] = toTopDBQuery(row, durationSec)
	}

	return PaginatedDBQueries{Results: results, PageInfo: pageInfo}, nil
}

func (s *Service) GetOperationBaseline(ctx context.Context, tenantID int64, startMs, endMs int64, serviceName, operationName string) (OperationBaseline, error) {
	row, err := s.repo.GetOperationBaseline(ctx, tenantID, startMs, endMs, serviceName, operationName)
	if err != nil {
		return OperationBaseline{}, err
	}
	return OperationBaseline{
		ServiceName:   serviceName,
		OperationName: operationName,
		P50Ms:         httputil.SanitizeFloat(float64(row.P50Ms)),
		P95Ms:         httputil.SanitizeFloat(float64(row.P95Ms)),
		P99Ms:         httputil.SanitizeFloat(float64(row.P99Ms)),
		SpanCount:     int64(row.SpanCount),
	}, nil
}
