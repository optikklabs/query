package infraconsts

// Error-rate percentages above which a host is degraded or unhealthy.
const (
	DegradedErrorPct  = 2.0
	UnhealthyErrorPct = 10.0
)

// RatioPct converts an OpenTelemetry utilization ratio (0..1) to a percentage.
func RatioPct(ratio float64) float64 {
	return ratio * PercentageMultiplier
}

// Mean returns the mean of values, or nil when there are none.
func Mean(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return new(sum / float64(len(values)))
}

// UsageFilterSQL keeps, of the per-state utilization series, only the one
// UsageValueSQL turns into usage: idle CPU and used memory.
const UsageFilterSQL = `NOT (metric_name = '` + MetricSystemCPUUtilization + `' AND attributes['state'] != 'idle')
		  AND NOT (metric_name = '` + MetricSystemMemoryUtilization + `' AND attributes['state'] != 'used')`

// UsageValueSQL is a metric group's mean ratio, with idle CPU inverted into
// busy CPU. Pair it with UsageFilterSQL and GROUP BY metric_name.
const UsageValueSQL = `if(metric_name = '` + MetricSystemCPUUtilization + `',
		       1 - sum(val_sum) / sum(val_count),
		       sum(val_sum) / sum(val_count))`
