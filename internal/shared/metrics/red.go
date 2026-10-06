package metrics

// REDDerivations returns the error rate (percent) and mean latency for a
// request count, error count and summed duration.
func REDDerivations(reqCount, errCount uint64, durationMsSum float64) (errorRate, avgLatencyMs float64) {
	return Percentage(errCount, reqCount), AvgLatency(durationMsSum, reqCount)
}
