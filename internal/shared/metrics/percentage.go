package metrics

// Percentage returns part as a percentage of whole, or 0 when whole is not
// positive.
func Percentage[T ~int64 | ~uint64](part, whole T) float64 {
	if whole <= 0 {
		return 0
	}
	return float64(part) * 100 / float64(whole)
}

// AvgLatency returns sumMs/count, or 0 when count is 0.
func AvgLatency(sumMs float64, count uint64) float64 {
	if count == 0 {
		return 0
	}
	return sumMs / float64(count)
}
