package latency

type LatencyTimeSeries struct {
	TimeBucket string   `json:"timeBucket"`
	GroupBy    string   `json:"groupBy"`
	P50Ms      *float64 `json:"p50Ms"`
	P95Ms      *float64 `json:"p95Ms"`
	P99Ms      *float64 `json:"p99Ms"`
}
