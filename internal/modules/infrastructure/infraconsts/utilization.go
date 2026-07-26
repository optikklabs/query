package infraconsts

import "math"

// NormalizeUtilization coerces a utilization reading to a 0..100 percentage,
// returning nil for values that cannot be one. Agents report either a 0..1
// fraction or an already-scaled percentage, and the two are told apart by
// magnitude against PercentageThreshold.
func NormalizeUtilization(v float64) *float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > PercentageThreshold*100 {
		return nil
	}
	if v <= PercentageThreshold {
		v *= PercentageMultiplier
	}
	return &v
}

// AverageUtilization means the values as given. Callers pass readings that
// NormalizeUtilization has already validated, so there is nothing left to
// filter; nil means nothing was present, which is distinct from an average of
// zero.
func AverageUtilization(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	avg := sum / float64(len(values))
	return &avg
}
