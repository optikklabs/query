package service

import (
	"context"

	"github.com/optikklabs/query/internal/infra/cursor"
	"github.com/optikklabs/query/internal/modules/services/redfleet/filter"
	"github.com/optikklabs/query/internal/modules/services/redfleet/models"
	"github.com/optikklabs/query/internal/shared/httputil"
)

func (s *Service) GetTopEndpointsCombined(
	ctx context.Context, f filter.Filters, limit int, cursorIn models.TopEndpointsCursor,
) (models.PaginatedEndpoints, error) {
	rows, err := s.repo.GetTopEndpointsCombined(ctx, f, limit+1, cursorIn)
	if err != nil {
		return models.PaginatedEndpoints{}, err
	}

	rows, pageInfo := cursor.Paginate(rows, limit, func(r models.TopEndpointRow) string {
		return cursor.Encode(models.TopEndpointsCursor{
			TotalCount: r.TotalCount, ServiceName: r.ServiceName, OperationName: r.OperationName,
			SpanKind: r.SpanKind, HTTPRoute: r.HTTPRoute, HTTPMethod: r.HTTPMethod, RPCSystem: r.RPCSystem,
		})
	})

	durationSec := windowSeconds(f)
	results := make([]models.TopEndpoint, len(rows))
	for i, row := range rows {
		results[i] = toTopEndpoint(row, durationSec)
	}

	return models.PaginatedEndpoints{Results: results, PageInfo: pageInfo}, nil
}

func (s *Service) GetTopDBQueries(
	ctx context.Context, f filter.Filters, limit int, cursorIn models.TopEndpointsCursor,
) (models.PaginatedDBQueries, error) {
	rows, err := s.repo.GetTopDBQueriesCombined(ctx, f, limit+1, cursorIn)
	if err != nil {
		return models.PaginatedDBQueries{}, err
	}

	rows, pageInfo := cursor.Paginate(rows, limit, func(r models.TopDBQueryRow) string {
		return cursor.Encode(models.TopEndpointsCursor{
			TotalCount: r.TotalCount, ServiceName: r.ServiceName,
			OperationName: r.OperationName, DBSystem: r.DBSystem,
		})
	})

	durationSec := windowSeconds(f)
	results := make([]models.TopDBQuery, len(rows))
	for i, row := range rows {
		results[i] = toTopDBQuery(row, durationSec)
	}

	return models.PaginatedDBQueries{Results: results, PageInfo: pageInfo}, nil
}

func (s *Service) GetOperationBaseline(
	ctx context.Context, tenantID int64, startMs, endMs int64, serviceName, operationName string,
) (models.OperationBaseline, error) {
	row, err := s.repo.GetOperationBaseline(ctx, tenantID, startMs, endMs, serviceName, operationName)
	if err != nil {
		return models.OperationBaseline{}, err
	}
	return models.OperationBaseline{
		ServiceName:   serviceName,
		OperationName: operationName,
		P50Ms:         httputil.SanitizeFloat(float64(row.P50Ms)),
		P95Ms:         httputil.SanitizeFloat(float64(row.P95Ms)),
		P99Ms:         httputil.SanitizeFloat(float64(row.P99Ms)),
		SpanCount:     int64(row.SpanCount),
	}, nil
}
