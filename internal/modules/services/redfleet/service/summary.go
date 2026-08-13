package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/services/redfleet/filter"
	"github.com/optikklabs/query/internal/modules/services/redfleet/models"
	"github.com/optikklabs/query/internal/shared/httputil"
	"golang.org/x/sync/errgroup"
)

var summaryMetrics = []string{
	infraconsts.MetricSystemCPUUtilization,
	infraconsts.MetricSystemCPUUsage,
	infraconsts.MetricProcessCPUUsage,
	infraconsts.MetricJVMCPUUtilization,
	infraconsts.MetricSystemMemoryUtilization,
	infraconsts.MetricSystemDiskUtilization,
}

var saturationSeriesMetrics = []string{
	infraconsts.MetricSystemCPUUtilization,
	infraconsts.MetricSystemCPUUsage,
	infraconsts.MetricProcessCPUUsage,
	infraconsts.MetricJVMCPUUtilization,
}

func (s *Service) GetServiceSummary(ctx context.Context, f filter.Filters) (models.ServiceSummaryResponse, error) {
	serviceName := f.SingleService()

	var (
		redRows []models.REDMetricsRow
		sats    []models.ServiceMetricRow
	)
	g, groupCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		redRows, err = s.repo.GetFleetREDMetrics(groupCtx, f)
		return err
	})
	g.Go(func() error {
		// Saturation is best-effort: degrade gracefully but never silently.
		rows, err := s.repo.GetServiceSaturationAggs(groupCtx, f.TenantID, f.StartMs, f.EndMs, serviceName, summaryMetrics)
		if err != nil {
			slog.WarnContext(ctx, "service summary: saturation query failed, omitting saturation metrics",
				slog.String("service", serviceName),
				slog.Any("error", err),
			)
			return nil
		}
		sats = rows
		return nil
	})
	if err := g.Wait(); err != nil {
		return models.ServiceSummaryResponse{}, err
	}

	var redRow *models.REDMetricsRow
	if len(redRows) > 0 {
		redRow = &redRows[0]
	}

	cpuVal, memVal, diskVal := extractSaturationAverages(sats)
	reqCount, errCount, rps, errRate, p50, p95, p99 := extractREDMetrics(redRow, windowSeconds(f))

	return models.ServiceSummaryResponse{
		ServiceName:       serviceName,
		RequestCount:      reqCount,
		ErrorCount:        errCount,
		RPS:               httputil.SanitizeFloat(rps),
		ErrorRate:         httputil.SanitizeFloat(errRate),
		P50Ms:             p50,
		P95Ms:             p95,
		P99Ms:             p99,
		CPUUtilization:    httputil.SanitizeFloat(cpuVal),
		MemoryUtilization: httputil.SanitizeFloat(memVal),
		DiskUtilization:   httputil.SanitizeFloat(diskVal),
	}, nil
}

func (s *Service) GetServiceSaturationTimeSeries(ctx context.Context, f filter.Filters) ([]models.SaturationTimeSeriesPoint, error) {
	rows, err := s.repo.GetServiceSaturationTimeSeries(ctx, f.TenantID, f.StartMs, f.EndMs, f.SingleService(), saturationSeriesMetrics)
	if err != nil {
		return nil, err
	}

	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	return timebucket.FillGaps(f.StartMs, f.EndMs, grain, rows,
		func(r models.SaturationPointRow) time.Time { return r.BucketAt },
		func(t time.Time, row models.SaturationPointRow, ok bool) models.SaturationTimeSeriesPoint {
			var val float64
			if ok {
				if normalized := infraconsts.NormalizeUtilization(row.Value); normalized != nil {
					val = *normalized
				}
			}
			return models.SaturationTimeSeriesPoint{
				TimestampMs: t.UnixMilli(),
				Value:       httputil.SanitizeFloat(val),
			}
		}), nil
}
