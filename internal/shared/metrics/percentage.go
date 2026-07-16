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
