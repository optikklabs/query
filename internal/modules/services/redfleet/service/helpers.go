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
			AvgLatency:   httputil.SanitizeFloat(float64(row.P50Ms)),
			P95Latency:   httputil.SanitizeFloat(float64(row.P95Ms)),
			P99Latency:   httputil.SanitizeFloat(float64(row.P99Ms)),
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
	avgErrorRate := metrics.PercentageInt(totalErrors, totalCount)
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

func extractSaturationAverages(sats []models.ServiceMetricRow) (cpuVal, memVal, diskVal float64) {
	var cpuValues []float64

	for _, row := range sats {
		switch row.MetricName {
		case infraconsts.MetricSystemCPUUtilization, infraconsts.MetricSystemCPUUsage, infraconsts.MetricProcessCPUUsage, infraconsts.MetricJVMCPUUtilization:
			if v := infraconsts.NormalizeUtilization(row.Value); v != nil {
				cpuValues = append(cpuValues, *v)
			}
		case infraconsts.MetricSystemMemoryUtilization:
			if v := infraconsts.NormalizeUtilization(row.Value); v != nil {
				memVal = *v
			}
		case infraconsts.MetricSystemDiskUtilization:
			if v := infraconsts.NormalizeUtilization(row.Value); v != nil {
				diskVal = *v
			}
		}
	}

	if cpuAvg := infraconsts.AverageUtilization(cpuValues); cpuAvg != nil {
		cpuVal = *cpuAvg
	}

	return cpuVal, memVal, diskVal
}

func extractREDMetrics(redRow *models.REDMetricsRow, durationSec float64) (reqCount, errCount int64, rps, errRate, p50, p95, p99 float64) {
	if redRow == nil {
		return
	}

	reqCount = int64(redRow.TotalCount)
	errCount = int64(redRow.ErrorCount)
	rps = float64(reqCount) / durationSec
	errRate = metrics.PercentageInt(errCount, reqCount)
	p50 = httputil.SanitizeFloat(float64(redRow.P50Ms))
	p95 = httputil.SanitizeFloat(float64(redRow.P95Ms))
	p99 = httputil.SanitizeFloat(float64(redRow.P99Ms))

	return
}
