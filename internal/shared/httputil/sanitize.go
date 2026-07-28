package httputil

import "math"

// SanitizeFloat zeroes NaN/Inf so computed metrics always JSON-encode.
func SanitizeFloat(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}
