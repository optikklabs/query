package redfleet

import (
	"context"
	"log/slog"
	"time"

	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/shared/httputil"
	"golang.org/x/sync/errgroup"
)

func (s *Service) GetServiceSummary(ctx context.Context, f REDFilters) (ServiceSummaryResponse, error) {
	serviceName := f.SingleService()
	metricNames := []string{
		infraconsts.MetricSystemCPUUtilization,
		infraconsts.MetricSystemCPUUsage,
		infraconsts.MetricProcessCPUUsage,
		infraconsts.MetricJVMCPUUtilization,
		infraconsts.MetricSystemMemoryUtilization,
		infraconsts.MetricSystemDiskUtilization,
	}

	var (
		redRows []redMetricsRow
		sats    []serviceMetricRow
	)
	g, groupCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		redRows, err = s.repo.GetFleetREDMetrics(groupCtx, f)
		return err
	})
	g.Go(func() error {
		// Saturation is best-effort: degrade gracefully but never silently.
		rows, err := s.repo.GetServiceSaturationAggs(groupCtx, f.TenantID, f.StartMs, f.EndMs, serviceName, metricNames)
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
		return ServiceSummaryResponse{}, err
	}

	var redRow *redMetricsRow
	if len(redRows) > 0 {
		redRow = &redRows[0]
	}

	cpuVal, memVal, diskVal := extractSaturationAverages(sats)

	durationSec := float64(f.EndMs-f.StartMs) / 1000.0
	if durationSec <= 0 {
		durationSec = 1
	}

	reqCount, errCount, rps, errRate, p50, p95, p99 := extractREDMetrics(redRow, durationSec)

	return ServiceSummaryResponse{
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

func (s *Service) GetServiceSaturationTimeSeries(ctx context.Context, f REDFilters) ([]SaturationTimeSeriesPoint, error) {
	serviceName := f.SingleService()
	metricNames := []string{
		infraconsts.MetricSystemCPUUtilization,
		infraconsts.MetricSystemCPUUsage,
		infraconsts.MetricProcessCPUUsage,
		infraconsts.MetricJVMCPUUtilization,
	}

	rows, err := s.repo.GetServiceSaturationTimeSeries(ctx, f.TenantID, f.StartMs, f.EndMs, serviceName, metricNames)
	if err != nil {
		return nil, err
	}

	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	return timebucket.FillGaps(f.StartMs, f.EndMs, grain, rows,
		func(r saturationTimeSeriesRawRow) time.Time { return r.BucketAt },
		func(t time.Time, row saturationTimeSeriesRawRow, ok bool) SaturationTimeSeriesPoint {
			var val float64
			if ok {
				if normalized := infraconsts.NormalizeUtilization(row.Value); normalized != nil {
					val = *normalized
				}
			}
			return SaturationTimeSeriesPoint{
				Timestamp: t,
				Value:     httputil.SanitizeFloat(val),
			}
		}), nil
}
