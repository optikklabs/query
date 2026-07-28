package metrics

func REDDerivations(reqCount, errCount uint64, durationMsSum float64) (errorRate, avgLatencyMs float64) {
	if reqCount == 0 {
		return 0, 0
	}
	return Percentage(errCount, reqCount), durationMsSum / float64(reqCount)
}
