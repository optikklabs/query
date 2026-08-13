package service

import (
	"context"

	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/saturation/database/models"
)

func percentilePair(qs []float32) (*float64, *float64) {
	if len(qs) < 2 {
		return nil, nil
	}
	p95, p99 := float64(qs[0]), float64(qs[1])
	return &p95, &p99
}

func (s *Service) GetQueryPerformanceCatalogue(
	ctx context.Context,
	tenantID, startMs, endMs int64,
	dbSystem string,
) (models.QueryPerformanceCatalogue, error) {
	collectionRows, queryRows, err := s.repo.GetQueryCatalogue(ctx, tenantID, startMs, endMs, dbSystem)
	if err != nil {
		return models.QueryPerformanceCatalogue{}, err
	}
	response := models.QueryPerformanceCatalogue{
		Collections: []models.QueryPerformanceCollection{},
		Queries:     []models.QueryPerformanceOption{},
	}
	for _, row := range collectionRows {
		p95, p99 := percentilePair(row.QS)
		response.Collections = append(response.Collections, models.QueryPerformanceCollection{
			Name:       row.Name,
			QueryCount: int64(row.QueryCount),
			CallCount:  int64(row.CallCount),
			P95Ms:      p95,
			P99Ms:      p99,
		})
	}
	for _, row := range queryRows {
		p95, p99 := percentilePair(row.QS)
		response.Queries = append(response.Queries, models.QueryPerformanceOption{
			QueryHash:      row.QueryHash,
			QueryLabel:     row.QueryLabel,
			CollectionName: row.CollectionName,
			CallCount:      int64(row.CallCount),
			P95Ms:          p95,
			P99Ms:          p99,
		})
	}
	if len(queryRows) > 0 {
		response.TotalQueries = int64(queryRows[0].TotalQueries)
	}
	response.Truncated = response.TotalQueries > int64(len(response.Queries))
	return response, nil
}

func (s *Service) GetQueryPerformanceSeries(
	ctx context.Context,
	tenantID, startMs, endMs int64,
	dbSystem, collection, queryHash string,
	limit int,
) (models.QueryPerformanceResponse, error) {
	queries, truncated, err := s.repo.GetRankedQueries(
		ctx, tenantID, startMs, endMs, dbSystem, collection, queryHash, limit,
	)
	if err != nil {
		return models.QueryPerformanceResponse{}, err
	}
	hashes := make([]string, len(queries))
	for i, query := range queries {
		hashes[i] = query.QueryHash
	}
	points, err := s.repo.GetQuerySeriesPoints(ctx, tenantID, startMs, endMs, dbSystem, collection, hashes)
	if err != nil {
		return models.QueryPerformanceResponse{}, err
	}
	response := models.QueryPerformanceResponse{
		BucketSizeSeconds: int64(timebucket.DisplayGrain(endMs - startMs).Seconds()),
		Series:            make([]models.QueryPerformanceSeries, len(queries)),
		Truncated:         truncated,
	}
	indexByHash := make(map[string]int, len(queries))
	for i, query := range queries {
		indexByHash[query.QueryHash] = i
		response.Series[i] = models.QueryPerformanceSeries{
			QueryHash:      query.QueryHash,
			QueryLabel:     query.QueryLabel,
			CollectionName: query.CollectionName,
			CallCount:      int64(query.CallCount),
			Points:         []models.QueryPerformancePoint{},
		}
	}
	for _, row := range points {
		index, ok := indexByHash[row.QueryHash]
		if !ok || len(row.QS) < 3 {
			continue
		}
		response.Series[index].Points = append(response.Series[index].Points, models.QueryPerformancePoint{
			TimeBucketMs: row.BucketAt.UnixMilli(),
			P50Ms:        float64(row.QS[0]),
			P95Ms:        float64(row.QS[1]),
			P99Ms:        float64(row.QS[2]),
			OpsPerSec:    row.OpsPerSec,
		})
	}
	return response, nil
}
