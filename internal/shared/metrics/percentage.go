package metrics

func Percentage(numerator, denominator uint64) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) * 100 / float64(denominator)
}

func PercentageInt(numerator, denominator int64) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) * 100 / float64(denominator)
}

func ComputeErrorRate(errs, total int64) float64 {
	return PercentageInt(errs, total)
}

func FacetPercentage(count, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(count) * 100.0 / float64(total)
}

func ComputeAvgLatency(sumMs float64, count uint64) float64 {
	if count == 0 {
		return 0
	}
	return sumMs / float64(count)
}
