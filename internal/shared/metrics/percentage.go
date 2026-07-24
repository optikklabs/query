// Package metrics contains small, unit-safe metric derivations shared by API modules.
package metrics

// Percentage returns numerator / denominator as a percentage in the inclusive
// range 0–100. Error-rate API fields use this unit throughout the query service.
func Percentage(numerator, denominator uint64) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) * 100 / float64(denominator)
}

// PercentageInt is Percentage for signed count values.
func PercentageInt(numerator, denominator int64) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) * 100 / float64(denominator)
}

// ComputeErrorRate returns error percentage for error and total counts.
func ComputeErrorRate(errs, total int64) float64 {
	return PercentageInt(errs, total)
}

// FacetPercentage computes relative percentage of a facet count against total.
func FacetPercentage(count, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(count) * 100.0 / float64(total)
}

// ComputeAvgLatency calculates average latency given total duration sum and count.
func ComputeAvgLatency(sumMs float64, count uint64) float64 {
	if count == 0 {
		return 0
	}
	return sumMs / float64(count)
}
