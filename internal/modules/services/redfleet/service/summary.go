package service

import (
	"context"

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
		var err error
		sats, err = s.repo.GetServiceSaturationAggs(groupCtx, f.TenantID, f.StartMs, f.EndMs, serviceName, summaryMetrics)
		return err
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
		CPUUtilization:    cpuVal,
		MemoryUtilization: memVal,
		DiskUtilization:   diskVal,
	}, nil
}

// GetServiceSaturationTimeSeries is the service's mean CPU utilization per
// bucket across the CPU metrics it and its hosts report.
func (s *Service) GetServiceSaturationTimeSeries(ctx context.Context, f filter.Filters) ([]models.SaturationTimeSeriesPoint, error) {
	rows, err := s.repo.GetServiceSaturationTimeSeries(ctx, f.TenantID, f.StartMs, f.EndMs, f.SingleService(), saturationSeriesMetrics)
	if err != nil {
		return nil, err
	}
	grain := timebucket.DisplayGrain(f.EndMs - f.StartMs)
	byBucket := make(map[int64][]float64)
	for _, row := range rows {
		key := row.BucketAt.UTC().Truncate(grain).Unix()
		byBucket[key] = append(byBucket[key], infraconsts.RatioPct(row.Value))
	}
	buckets := timebucket.DenseBuckets(f.StartMs, f.EndMs, grain)
	out := make([]models.SaturationTimeSeriesPoint, len(buckets))
	for i, t := range buckets {
		out[i] = models.SaturationTimeSeriesPoint{TimestampMs: t.UnixMilli(), Value: infraconsts.Mean(byBucket[t.Unix()])}
	}
	return out, nil
}
