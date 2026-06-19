package redfleet

import "time"

// FleetTotals is the fleet-wide RED rollup powering the overview hero KPIs.
type FleetTotals struct {
	ServiceCount   int64   `json:"service_count"`
	TotalSpanCount int64   `json:"total_span_count"`
	TotalErrors    int64   `json:"total_errors"`
	TotalRPS       float64 `json:"total_rps"`
	AvgErrorPct    float64 `json:"avg_error_pct"`
	AvgP50Ms       float64 `json:"avg_p50_ms"`
	AvgP95Ms       float64 `json:"avg_p95_ms"`
	AvgP99Ms       float64 `json:"avg_p99_ms"`
}

// ServiceREDMetric is one per-service RED row for the fleet services list.
type ServiceREDMetric struct {
	ServiceName  string  `json:"service_name"`
	RequestCount int64   `json:"request_count"`
	ErrorCount   int64   `json:"error_count"`
	AvgLatency   float64 `json:"avg_latency"`
	P95Latency   float64 `json:"p95_latency"`
	P99Latency   float64 `json:"p99_latency"`
}

type ApdexScore struct {
	ServiceName string  `json:"service_name"`
	Apdex       float64 `json:"apdex"`
	Satisfied   int64   `json:"satisfied"`
	Tolerating  int64   `json:"tolerating"`
	Frustrated  int64   `json:"frustrated"`
	TotalCount  int64   `json:"total_count"`
}

type ServicePerformancePoint struct {
	Timestamp    time.Time `json:"timestamp"    ch:"timestamp"`
	RPS          float64   `json:"rps"          ch:"rps"`
	RequestCount uint64    `json:"request_count"`
	ErrorCount   uint64    `json:"error_count"`
	ErrorPct     float64   `json:"error_pct"`
}

// StatusTimeSeriesPoint is one display-bucket row with span counts split by
// the OTel http.status_code bucket (`2xx` / `4xx` / `5xx`).
type StatusTimeSeriesPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	Status2xx   float64   `json:"status_2xx"`
	Status4xx   float64   `json:"status_4xx"`
	Status5xx   float64   `json:"status_5xx"`
	StatusOther float64   `json:"status_other"`
}

// LatencyPercentilesPoint is one display-bucket row with p50/p95/p99 latency.
type LatencyPercentilesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	P50Ms     float64   `json:"p50_ms"`
	P95Ms     float64   `json:"p95_ms"`
	P99Ms     float64   `json:"p99_ms"`
}

// TopEndpoint is one per-operation row used by the *Service Detail endpoints
// table — combines rate, error %, and p50/p95/p99 latency.
type TopEndpoint struct {
	OperationName string  `json:"operation_name"`
	ServiceName   string  `json:"service_name"`
	SpanKind      string  `json:"span_kind"`
	HTTPRoute     string  `json:"http_route"`
	RPS           float64 `json:"rps"`
	ErrorRate     float64 `json:"error_rate"`
	ErrorCount    int64   `json:"error_count"`
	TotalCount    int64   `json:"total_count"`
	P50Ms         float64 `json:"p50_ms"`
	P95Ms         float64 `json:"p95_ms"`
	P99Ms         float64 `json:"p99_ms"`
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

type apdexRow struct {
	ServiceName string `ch:"service"`
	TotalCount  uint64 `ch:"total_count"`
	Satisfied   uint64 `ch:"satisfied"`
	Tolerating  uint64 `ch:"tolerating"`
}
