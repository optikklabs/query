package errors

import (
	"context"
	"time"

	"github.com/optikklabs/query/internal/infra/cursor"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/metrics"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetServiceErrorRate(ctx context.Context, tenantID int64, startMs, endMs int64, serviceName string) ([]TimeSeriesPoint, error) {
	raw, err := s.repo.ServiceErrorRateRows(ctx, tenantID, startMs, endMs, serviceName)
	if err != nil {
		return nil, err
	}
	return fillServicePoints(startMs, endMs, serviceName, raw,
		func(t time.Time, row rawServiceRateRow, _ bool) TimeSeriesPoint {
			total, errs := int64(row.RequestCount), int64(row.ErrorCount)
			return TimeSeriesPoint{
				Timestamp:    t,
				RequestCount: total,
				ErrorCount:   errs,
				ErrorRate:    metrics.ComputeErrorRate(errs, total),
				AvgLatency:   metrics.ComputeAvgLatency(row.DurationMsSum, row.RequestCount),
			}
		}), nil
}

// fillServicePoints densifies rate rows into per-service series: one series
// for a named service, or one per non-empty service in the rows otherwise.
func fillServicePoints(
	startMs, endMs int64, serviceName string, raw []rawServiceRateRow,
	point func(t time.Time, row rawServiceRateRow, ok bool) TimeSeriesPoint,
) []TimeSeriesPoint {
	grain := timebucket.DisplayGrainForRange(startMs, endMs)
	at := func(r rawServiceRateRow) time.Time { return r.BucketAt }

	if serviceName != "" {
		points := timebucket.FillGaps(startMs, endMs, grain, raw, at, point)
		for i := range points {
			points[i].ServiceName = serviceName
		}
		return points
	}

	named := make([]rawServiceRateRow, 0, len(raw))
	for _, row := range raw {
		if row.ServiceName != "" {
			named = append(named, row)
		}
	}
	services, _, series := timebucket.FillGapsKeyed(startMs, endMs, grain, named,
		func(r rawServiceRateRow) string { return r.ServiceName }, at, point)
	var points []TimeSeriesPoint
	for i, svc := range services {
		for j := range series[i] {
			series[i][j].ServiceName = svc
		}
		points = append(points, series[i]...)
	}
	return points
}

func (s *Service) GetErrorGroupDetail(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string) (*ErrorGroupDetail, error) {
	row, err := s.repo.ErrorGroupDetailRow(ctx, tenantID, startMs, endMs, groupID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return &ErrorGroupDetail{
		GroupID:         groupID,
		ServiceName:     row.ServiceName,
		OperationName:   row.OperationName,
		HTTPStatusCode:  int(row.HTTPStatusCode),
		ErrorCount:      int64(row.ErrorCount),
		LastOccurrence:  row.LastOccurrence,
		FirstOccurrence: row.FirstOccurrence,
		ExceptionType:   row.ExceptionType,
	}, nil
}

var facetColumns = []string{"service_version", "environment", "pod", "http_route"}

func (s *Service) GetErrorGroupLatestOccurrence(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string) (*ErrorLatestOccurrence, error) {
	row, err := s.repo.ErrorGroupLatestOccurrenceRow(ctx, tenantID, startMs, endMs, groupID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return &ErrorLatestOccurrence{
		TraceID:        row.TraceID,
		SpanID:         row.SpanID,
		Timestamp:      row.Timestamp,
		DurationMs:     row.DurationMs,
		Message:        row.ExceptionMessage,
		Stacktrace:     row.StackTrace,
		HTTPMethod:     row.HTTPMethod,
		HTTPRoute:      row.HTTPRoute,
		HTTPStatusCode: row.HTTPStatusCode,
		ServiceVersion: row.ServiceVersion,
		Environment:    row.Environment,
		Pod:            row.Pod,
		Host:           row.Host,
	}, nil
}

func (s *Service) GetErrorGroupFacets(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string) ([]ErrorFacetGroup, error) {
	rows, err := s.repo.ErrorGroupFacetRowsAll(ctx, tenantID, startMs, endMs, groupID)
	if err != nil {
		return nil, err
	}

	byDim := make(map[string][]rawErrorFacetGroupRow)
	totDim := make(map[string]int64)
	for _, r := range rows {
		byDim[r.Dim] = append(byDim[r.Dim], r)
		totDim[r.Dim] += int64(r.Count)
	}

	groups := make([]ErrorFacetGroup, 0, len(facetColumns))
	for _, col := range facetColumns {
		dimRows := byDim[col]
		if len(dimRows) == 0 {
			continue
		}
		total := totDim[col]
		facets := make([]ErrorFacet, len(dimRows))
		for i, r := range dimRows {
			cnt := int64(r.Count)
			facets[i] = ErrorFacet{
				Name:  r.Value,
				Count: cnt,
				Pct:   metrics.FacetPercentage(cnt, total),
			}
		}
		groups = append(groups, ErrorFacetGroup{Key: col, Facets: facets})
	}
	return groups, nil
}

func (s *Service) GetErrorGroupTraces(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string, limit int, cursorIn ErrorTracesCursor) (PaginatedErrorTraces, error) {
	raw, err := s.repo.ErrorGroupTraceRows(ctx, tenantID, startMs, endMs, groupID, limit+1, cursorIn)
	if err != nil {
		return PaginatedErrorTraces{}, err
	}
	raw, pageInfo := cursor.Paginate(raw, limit, func(r rawErrorGroupTraceRow) string {
		return cursor.Encode(ErrorTracesCursor{Timestamp: r.Timestamp, SpanID: r.SpanID})
	})
	traces := make([]ErrorGroupTrace, len(raw))
	for i, row := range raw {
		traces[i] = ErrorGroupTrace{
			TraceID:    row.TraceID,
			SpanID:     row.SpanID,
			Timestamp:  row.Timestamp,
			DurationMs: row.DurationMs,
			StatusCode: row.StatusCode,
		}
	}
	return PaginatedErrorTraces{Results: traces, PageInfo: pageInfo}, nil
}

func (s *Service) GetErrorGroupTimeseries(ctx context.Context, tenantID int64, startMs, endMs int64, groupID string) ([]TimeSeriesPoint, error) {
	raw, err := s.repo.ErrorGroupTimeseriesRows(ctx, tenantID, startMs, endMs, groupID)
	if err != nil {
		return nil, err
	}

	grain := timebucket.DisplayGrain(endMs - startMs)
	return timebucket.FillGaps(startMs, endMs, grain, raw,
		func(r rawTimeBucketCountRow) time.Time { return r.BucketAt },
		func(t time.Time, row rawTimeBucketCountRow, _ bool) TimeSeriesPoint {
			return TimeSeriesPoint{Timestamp: t, ErrorCount: int64(row.Count)}
		}), nil
}

func (s *Service) GetErrorHotspot(ctx context.Context, tenantID int64, startMs, endMs int64) ([]ErrorHotspotCell, error) {
	raw, err := s.repo.ErrorHotspotRows(ctx, tenantID, startMs, endMs)
	if err != nil {
		return nil, err
	}
	cells := make([]ErrorHotspotCell, len(raw))
	for i, row := range raw {
		cells[i] = ErrorHotspotCell{
			ServiceName:   row.ServiceName,
			OperationName: row.OperationName,
			GroupID:       row.GroupID,
			ErrorCount:    int64(row.ErrorCount),
		}
	}
	return cells, nil
}
