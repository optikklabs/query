package topology

// nodeAggRow is scanned from the per-service RED aggregation query.
type nodeAggRow struct {
	ServiceName  string    `ch:"service"`
	RequestCount uint64    `ch:"request_count"`
	ErrorCount   uint64    `ch:"error_count"`
	QS           []float64 `ch:"qs"`
	P50Ms        float32   `ch:"p50_ms"`
	P95Ms        float32   `ch:"p95_ms"`
	P99Ms        float32   `ch:"p99_ms"`
}

type edgeAggRow struct {
	Source     string    `ch:"source"`
	Target     string    `ch:"target"`
	CallCount  uint64    `ch:"call_count"`
	ErrorCount uint64    `ch:"error_count"`
	QS         []float64 `ch:"qs"`
	P50Ms      float32   `ch:"p50_ms"`
	P95Ms      float32   `ch:"p95_ms"`
}
