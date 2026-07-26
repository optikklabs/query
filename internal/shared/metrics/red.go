package metrics

// REDDerivations returns the error percentage and mean latency for one
// span_stats aggregate, guarding the zero-request case that would otherwise
// divide by zero. It is the pair of Percentage and ComputeAvgLatency that
// every per-resource RED fold needs.
func REDDerivations(reqCount, errCount uint64, durationMsSum float64) (errorRate, avgLatencyMs float64) {
	if reqCount == 0 {
		return 0, 0
	}
	return Percentage(errCount, reqCount), durationMsSum / float64(reqCount)
}
