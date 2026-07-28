package infraconsts

import "math"

func NormalizeUtilization(v float64) *float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > PercentageThreshold*100 {
		return nil
	}
	if v <= PercentageThreshold {
		v *= PercentageMultiplier
	}
	return &v
}

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
