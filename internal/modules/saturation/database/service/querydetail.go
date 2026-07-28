package service

import (
	"context"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/saturation/database/filter"
	"github.com/optikklabs/query/internal/modules/saturation/database/models"
)

func (s *Service) GetSummary(ctx context.Context, tenantID, startMs, endMs int64, hash string, f filter.Filters) (*models.QuerySummary, error) {
	raw, err := s.repo.GetSummary(ctx, tenantID, startMs, endMs, hash, f)
	if err != nil || raw == nil {
		return nil, err
	}
	out := &models.QuerySummary{
		QueryHash:      hash,
		QueryText:      raw.QueryText,
		DbSystem:       raw.DbSystem,
		CollectionName: raw.CollectionName,
		OperationName:  operationName(raw.OperationName, raw.QueryText),
		CallCount:      int64(raw.CallCount),
		ErrorCount:     int64(raw.ErrorCount),
		AvgRows:        raw.AvgRows,
		Services:       []models.ServiceCalls{},
	}
	if raw.CallCount == 0 {
		return out, nil
	}
	if len(raw.QS) >= 3 {
		p50, p95, p99 := float64(raw.QS[0]), float64(raw.QS[1]), float64(raw.QS[2])
		out.P50Ms, out.P95Ms, out.P99Ms = &p50, &p95, &p99
	}
	out.AvgMs = raw.AvgMs
	out.TotalTimeMs = raw.TotalTimeMs

	services, err := s.repo.GetServices(ctx, tenantID, startMs, endMs, hash, f)
	if err != nil {
		return nil, err
	}
	for _, sv := range services {
		out.Services = append(out.Services, models.ServiceCalls{Service: sv.Service, CallCount: int64(sv.CallCount)})
	}
	return out, nil
}

func operationName(attr, queryText string) string {
	if attr != "" {
		return attr
	}
	if fields := strings.Fields(queryText); len(fields) > 0 {
		return strings.ToUpper(fields[0])
	}
	return ""
}

func (s *Service) GetTimeseries(ctx context.Context, tenantID, startMs, endMs int64, hash string, f filter.Filters) ([]models.QueryTimeseriesPoint, error) {
	rows, err := s.repo.GetTimeseries(ctx, tenantID, startMs, endMs, hash, f)
	if err != nil {
		return nil, err
	}
	out := make([]models.QueryTimeseriesPoint, len(rows))
	for i, r := range rows {
		avg, p99 := r.AvgMs, float64(r.P99Ms)
		out[i] = models.QueryTimeseriesPoint{
			TimeBucket: timebucket.FormatDisplayBucket(r.BucketAt),
			CallCount:  int64(r.CallCount),
			ErrorCount: int64(r.ErrorCount),
			AvgMs:      &avg,
			P99Ms:      &p99,
		}
	}
	return out, nil
}

func (s *Service) GetExecutions(ctx context.Context, tenantID, startMs, endMs int64, hash string, f filter.Filters, limit int) ([]models.QueryExecution, error) {
	rows, err := s.repo.GetExecutions(ctx, tenantID, startMs, endMs, hash, f, limit)
	if err != nil {
		return nil, err
	}
	out := make([]models.QueryExecution, len(rows))
	for i, r := range rows {
		out[i] = models.QueryExecution{
			Timestamp:  r.Timestamp.UTC().Format(time.RFC3339Nano),
			TraceID:    r.TraceID,
			SpanID:     r.SpanID,
			DurationMs: r.DurationMs,
			IsError:    r.IsError != 0,
			Service:    r.Service,
			Host:       r.Host,
			Rows:       r.Rows,
		}
	}
	return out, nil
}
