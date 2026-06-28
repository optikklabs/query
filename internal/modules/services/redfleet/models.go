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

// FleetOverviewResponse combines totals and per-service RED metrics in a single
// payload so the frontend needs only one request instead of two.
type FleetOverviewResponse struct {
	Totals   FleetTotals        `json:"totals"`
	Services []ServiceREDMetric `json:"services"`
}


type ServiceREDMetric struct {
	ServiceName  string  `json:"service_name"`
	RequestCount int64   `json:"request_count"`
	ErrorCount   int64   `json:"error_count"`
	AvgLatency   float64 `json:"avg_latency"`
	P95Latency   float64 `json:"p95_latency"`
	P99Latency   float64 `json:"p99_latency"`
}



type ServicePerformancePoint struct {
	Timestamp    time.Time `json:"timestamp"    ch:"timestamp"`
	RPS          float64   `json:"rps"          ch:"rps"`
	RequestCount uint64    `json:"request_count"`
	ErrorCount   uint64    `json:"error_count"`
	ErrorPct     float64   `json:"error_pct"`
}

type StatusTimeSeriesPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	Status2xx   float64   `json:"status_2xx"`
	Status4xx   float64   `json:"status_4xx"`
	Status5xx   float64   `json:"status_5xx"`
	StatusOther float64   `json:"status_other"`
}

type LatencyPercentilesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	P50Ms     float64   `json:"p50_ms"`
	P95Ms     float64   `json:"p95_ms"`
	P99Ms     float64   `json:"p99_ms"`
}

type EndpointRatePoint struct {
	Timestamp time.Time `json:"timestamp"`
	HTTPRoute string    `json:"http_route"`
	RPS       float64   `json:"rps"`
	ErrorRate *float64  `json:"error_rate"`
	P99Ms     *float64  `json:"p99_ms"`
}

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

type TopDBQuery struct {
	OperationName string  `json:"operation_name"`
	ServiceName   string  `json:"service_name"`
	DBSystem      string  `json:"db_system"`
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
	QS          []float64 `ch:"qs"`
	P50Ms       float32   `ch:"p50_ms"`
	P95Ms       float32   `ch:"p95_ms"`
	P99Ms       float32   `ch:"p99_ms"`
}


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

type RequestRatePoint struct {
	Timestamp   time.Time `json:"timestamp"    ch:"bucket_at"`
	ServiceName string    `json:"service_name" ch:"service_name"`
	RPS         float64   `json:"rps"`
}

type operationBaselineRow struct {
	SpanCount uint64    `ch:"span_count"`
	QS        []float64 `ch:"qs"`
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
