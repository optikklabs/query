package service

import (
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/services/redfleet/filter"
	"github.com/optikklabs/query/internal/modules/services/redfleet/models"
	"github.com/optikklabs/query/internal/shared/httputil"
	"github.com/optikklabs/query/internal/shared/metrics"
)

func windowSeconds(f filter.Filters) float64 {
	if sec := float64(f.EndMs-f.StartMs) / 1000.0; sec > 0 {
		return sec
	}
	return 1
}

func mapFleetServices(rows []models.REDMetricsRow) []models.ServiceREDMetric {
	services := make([]models.ServiceREDMetric, 0, len(rows))
	for _, row := range rows {
		if row.IsTotal != 0 {
			continue
		}
		services = append(services, models.ServiceREDMetric{
			ServiceName:  row.ServiceName,
			RequestCount: int64(row.TotalCount),
			ErrorCount:   int64(row.ErrorCount),
			P50Latency:   httputil.SanitizeFloat(float64(row.P50Ms)),
			P95Latency:   httputil.SanitizeFloat(float64(row.P95Ms)),
			P99Latency:   httputil.SanitizeFloat(float64(row.P99Ms)),
			Version:      row.Version,
			Environment:  row.Environment,
			Instances:    row.Instances,
		})
	}
	return services
}

func fleetTotalRow(rows []models.REDMetricsRow) *models.REDMetricsRow {
	for i := range rows {
		if rows[i].IsTotal != 0 {
			return &rows[i]
		}
	}
	return nil
}

func computeFleetTotals(total *models.REDMetricsRow, serviceCount int, startMs, endMs int64) models.FleetTotals {
	durationSec := float64(endMs-startMs) / 1000.0
	if durationSec <= 0 {
		durationSec = 1
	}

	if total == nil {
		return models.FleetTotals{ServiceCount: int64(serviceCount)}
	}
	totalCount := int64(total.TotalCount)
	totalErrors := int64(total.ErrorCount)
	avgErrorRate := metrics.Percentage(totalErrors, totalCount)
	return models.FleetTotals{
		ServiceCount:   int64(serviceCount),
		TotalSpanCount: totalCount,
		TotalErrors:    totalErrors,
		TotalRPS:       httputil.SanitizeFloat(float64(totalCount) / durationSec),
		AvgErrorRate:   httputil.SanitizeFloat(avgErrorRate),
		AvgP50Ms:       httputil.SanitizeFloat(float64(total.P50Ms)),
		AvgP95Ms:       httputil.SanitizeFloat(float64(total.P95Ms)),
		AvgP99Ms:       httputil.SanitizeFloat(float64(total.P99Ms)),
	}
}

func toTopDBQuery(row models.TopDBQueryRow, durationSec float64) models.TopDBQuery {
	return models.TopDBQuery{
		OperationName: row.OperationName,
		ServiceName:   row.ServiceName,
		DBSystem:      row.DBSystem,
		REDMetrics:    redMetrics(row.TotalCount, row.ErrorCount, row.P50Ms, row.P95Ms, row.P99Ms, durationSec),
	}
}

func toTopEndpoint(row models.TopEndpointRow, durationSec float64) models.TopEndpoint {
	return models.TopEndpoint{
		OperationName: row.OperationName,
		ServiceName:   row.ServiceName,
		SpanKind:      row.SpanKind,
		HTTPRoute:     row.HTTPRoute,
		HTTPMethod:    row.HTTPMethod,
		RPCSystem:     row.RPCSystem,
		REDMetrics:    redMetrics(row.TotalCount, row.ErrorCount, row.P50Ms, row.P95Ms, row.P99Ms, durationSec),
	}
}

func redMetrics(total, errors uint64, p50, p95, p99 float32, durationSec float64) models.REDMetrics {
	return models.REDMetrics{
		RPS: float64(total) / durationSec, ErrorRate: metrics.Percentage(errors, total),
		ErrorCount: int64(errors), TotalCount: int64(total),
		P50Ms: httputil.SanitizeFloat(float64(p50)), P95Ms: httputil.SanitizeFloat(float64(p95)), P99Ms: httputil.SanitizeFloat(float64(p99)),
	}
}

// extractSaturationAverages folds the summary's usage ratios into
// percentages: CPU is the mean of the CPU metrics reported.
func extractSaturationAverages(sats []models.ServiceMetricRow) (cpu, mem, disk *float64) {
	var cpuValues []float64
	for _, row := range sats {
		pct := infraconsts.RatioPct(row.Value)
		switch row.MetricName {
		case infraconsts.MetricSystemCPUUtilization, infraconsts.MetricSystemCPUUsage, infraconsts.MetricProcessCPUUsage, infraconsts.MetricJVMCPUUtilization:
			cpuValues = append(cpuValues, pct)
		case infraconsts.MetricSystemMemoryUtilization:
			mem = new(pct)
		case infraconsts.MetricSystemDiskUtilization:
			disk = new(pct)
		}
	}
	return infraconsts.Mean(cpuValues), mem, disk
}
