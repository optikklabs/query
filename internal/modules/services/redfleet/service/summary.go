package service

import (
	"context"

	"golang.org/x/sync/errgroup"

	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/services/redfleet/filter"
	"github.com/optikklabs/query/internal/modules/services/redfleet/models"
	"github.com/optikklabs/query/internal/shared/httputil"
	"github.com/optikklabs/query/internal/shared/metrics"
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

	resp := models.ServiceSummaryResponse{ServiceName: serviceName}
	resp.CPUUtilization, resp.MemoryUtilization, resp.DiskUtilization = extractSaturationAverages(sats)
	if len(redRows) > 0 {
		row := redRows[0]
		resp.RequestCount = int64(row.TotalCount)
		resp.ErrorCount = int64(row.ErrorCount)
		resp.RPS = httputil.SanitizeFloat(float64(resp.RequestCount) / windowSeconds(f))
		resp.ErrorRate = httputil.SanitizeFloat(metrics.Percentage(resp.ErrorCount, resp.RequestCount))
		resp.P50Ms = httputil.SanitizeFloat(row.P50Ms)
		resp.P95Ms = httputil.SanitizeFloat(row.P95Ms)
		resp.P99Ms = httputil.SanitizeFloat(row.P99Ms)
	}
	return resp, nil
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
