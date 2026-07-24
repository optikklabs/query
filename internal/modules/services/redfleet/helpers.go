package redfleet

import (
	"math"

	"github.com/optikklabs/query/internal/infra/utils"
	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/shared/metrics"
)

func mapFleetServices(rows []redMetricsRow) []ServiceREDMetric {
	services := make([]ServiceREDMetric, 0, len(rows))
	for _, row := range rows {
		if row.IsTotal != 0 {
			continue
		}
		services = append(services, ServiceREDMetric{
			ServiceName:  row.ServiceName,
			RequestCount: int64(row.TotalCount),
			ErrorCount:   int64(row.ErrorCount),
			AvgLatency:   utils.SanitizeFloat(float64(row.P50Ms)),
			P95Latency:   utils.SanitizeFloat(float64(row.P95Ms)),
			P99Latency:   utils.SanitizeFloat(float64(row.P99Ms)),
		})
	}
	return services
}

func fleetTotalRow(rows []redMetricsRow) *redMetricsRow {
	for i := range rows {
		if rows[i].IsTotal != 0 {
			return &rows[i]
		}
	}
	return nil
}

func computeFleetTotals(total *redMetricsRow, serviceCount int, startMs, endMs int64) FleetTotals {
	durationSec := float64(endMs-startMs) / 1000.0
	if durationSec <= 0 {
		durationSec = 1
	}

	if total == nil {
		return FleetTotals{ServiceCount: int64(serviceCount)}
	}
	totalCount := int64(total.TotalCount)
	totalErrors := int64(total.ErrorCount)
	avgErrorRate := metrics.PercentageInt(totalErrors, totalCount)
	return FleetTotals{
		ServiceCount:   int64(serviceCount),
		TotalSpanCount: totalCount,
		TotalErrors:    totalErrors,
		TotalRPS:       utils.SanitizeFloat(float64(totalCount) / durationSec),
		AvgErrorRate:   utils.SanitizeFloat(avgErrorRate),
		AvgP50Ms:       utils.SanitizeFloat(float64(total.P50Ms)),
		AvgP95Ms:       utils.SanitizeFloat(float64(total.P95Ms)),
		AvgP99Ms:       utils.SanitizeFloat(float64(total.P99Ms)),
	}
}

func writeStatusCount(pt *StatusTimeSeriesPoint, bucket string, count float64) {
	switch bucket {
	case "2xx":
		pt.Status2xx += count
	case "4xx":
		pt.Status4xx += count
	case "5xx":
		pt.Status5xx += count
	default:
		pt.StatusOther += count
	}
}

func toTopDBQuery(row topDBQueryRow, durationSec float64) TopDBQuery {
	total := int64(row.TotalCount)
	errs := int64(row.ErrorCount)
	errRate := metrics.PercentageInt(errs, total)
	return TopDBQuery{
		OperationName: row.OperationName,
		ServiceName:   row.ServiceName,
		DBSystem:      row.DBSystem,
		RPS:           float64(total) / durationSec,
		ErrorRate:     errRate,
		ErrorCount:    errs,
		TotalCount:    total,
		P50Ms:         utils.SanitizeFloat(float64(row.P50Ms)),
		P95Ms:         utils.SanitizeFloat(float64(row.P95Ms)),
		P99Ms:         utils.SanitizeFloat(float64(row.P99Ms)),
	}
}

func toTopEndpoint(row topEndpointRow, durationSec float64) TopEndpoint {
	total := int64(row.TotalCount)
	errs := int64(row.ErrorCount)
	errRate := metrics.PercentageInt(errs, total)
	return TopEndpoint{
		OperationName: row.OperationName,
		ServiceName:   row.ServiceName,
		SpanKind:      row.SpanKind,
		HTTPRoute:     row.HTTPRoute,
		RPS:           float64(total) / durationSec,
		ErrorRate:     errRate,
		ErrorCount:    errs,
		TotalCount:    total,
		P50Ms:         utils.SanitizeFloat(float64(row.P50Ms)),
		P95Ms:         utils.SanitizeFloat(float64(row.P95Ms)),
		P99Ms:         utils.SanitizeFloat(float64(row.P99Ms)),
	}
}

func extractSaturationAverages(sats []serviceMetricRow) (cpuVal, memVal, diskVal float64) {
	var cpuValues []float64

	for _, row := range sats {
		switch row.MetricName {
		case infraconsts.MetricSystemCPUUtilization, infraconsts.MetricSystemCPUUsage, infraconsts.MetricProcessCPUUsage, infraconsts.MetricJVMCPUUtilization:
			if v := normalizeUtilization(row.Value); v != nil {
				cpuValues = append(cpuValues, *v)
			}
		case infraconsts.MetricSystemMemoryUtilization:
			if v := normalizeUtilization(row.Value); v != nil {
				memVal = *v
			}
		case infraconsts.MetricSystemDiskUtilization:
			if v := normalizeUtilization(row.Value); v != nil {
				diskVal = *v
			}
		}
	}

	if cpuAvg := averageFloats(cpuValues); cpuAvg != nil {
		cpuVal = *cpuAvg
	}

	return cpuVal, memVal, diskVal
}

func extractREDMetrics(redRow *redMetricsRow, durationSec float64) (reqCount, errCount int64, rps, errRate, p50, p95, p99 float64) {
	if redRow == nil {
		return
	}

	reqCount = int64(redRow.TotalCount)
	errCount = int64(redRow.ErrorCount)
	rps = float64(reqCount) / durationSec
	errRate = metrics.PercentageInt(errCount, reqCount)
	p50 = utils.SanitizeFloat(float64(redRow.P50Ms))
	p95 = utils.SanitizeFloat(float64(redRow.P95Ms))
	p99 = utils.SanitizeFloat(float64(redRow.P99Ms))

	return
}

func normalizeUtilization(v float64) *float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > infraconsts.PercentageThreshold*100 {
		return nil
	}
	if v <= infraconsts.PercentageThreshold {
		v = v * infraconsts.PercentageMultiplier
	}
	return &v
}

func averageFloats(vals []float64) *float64 {
	if len(vals) == 0 {
		return nil
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	avg := sum / float64(len(vals))
	return &avg
}
