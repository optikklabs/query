package redservice

import "time"

// OperationBaseline is the windowed p50/p95/p99 for a single service+operation,
// powering the Trace Detail Duration card's "N× slower than p50" comparison.
type OperationBaseline struct {
	ServiceName   string  `json:"service_name"`
	OperationName string  `json:"operation_name"`
	P50Ms         float64 `json:"p50_ms"`
	P95Ms         float64 `json:"p95_ms"`
	P99Ms         float64 `json:"p99_ms"`
	SpanCount     int64   `json:"span_count"`
}

type ServiceSummaryResponse struct {
	ServiceName       string  `json:"service_name"`
	RequestCount      int64   `json:"request_count"`
	ErrorCount        int64   `json:"error_count"`
	RPS               float64 `json:"rps"`
	ErrorRate         float64 `json:"error_rate"`
	P50Ms             float64 `json:"p50_ms"`
	P95Ms             float64 `json:"p95_ms"`
	P99Ms             float64 `json:"p99_ms"`
	CPUUtilization    float64 `json:"cpu_utilization"`
	MemoryUtilization float64 `json:"memory_utilization"`
	DiskUtilization   float64 `json:"disk_utilization"`
}

type SaturationTimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

type redMetricsRow struct {
	ServiceName string    `ch:"service"`
	TotalCount  uint64    `ch:"total_count"`
	ErrorCount  uint64    `ch:"error_count"`
	HistTuple   []any     `ch:"hist"`
	QS          []float32 `ch:"qs"`
	P50Ms       float32   `ch:"p50_ms"`
	P95Ms       float32   `ch:"p95_ms"`
	P99Ms       float32   `ch:"p99_ms"`
}

type operationBaselineRow struct {
	SpanCount uint64    `ch:"span_count"`
	HistTuple []any     `ch:"hist"`
	QS        []float32 `ch:"qs"`
	P50Ms     float32   `ch:"p50_ms"`
	P95Ms     float32   `ch:"p95_ms"`
	P99Ms     float32   `ch:"p99_ms"`
}

type serviceMetricRow struct {
	Service    string  `ch:"service"`
	MetricName string  `ch:"metric_name"`
	Value      float64 `ch:"value"`
}

type saturationTimeSeriesRawRow struct {
	BucketAt time.Time `ch:"bucket_at"`
	Value    float64   `ch:"value"`
}
