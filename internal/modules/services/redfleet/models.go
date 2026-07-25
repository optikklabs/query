package redfleet

import "time"

// FleetTotals is the fleet-wide RED rollup powering the overview hero KPIs.
type FleetTotals struct {
	ServiceCount   int64   `json:"serviceCount"`
	TotalSpanCount int64   `json:"totalSpanCount"`
	TotalErrors    int64   `json:"totalErrors"`
	TotalRPS       float64 `json:"totalRps"`
	AvgErrorRate   float64 `json:"avgErrorRate"`
	AvgP50Ms       float64 `json:"avgP50Ms"`
	AvgP95Ms       float64 `json:"avgP95Ms"`
	AvgP99Ms       float64 `json:"avgP99Ms"`
}

// FleetOverviewResponse combines totals and per-service RED metrics in a single
// payload so the frontend needs only one request instead of two.
type FleetOverviewResponse struct {
	Totals   FleetTotals        `json:"totals"`
	Services []ServiceREDMetric `json:"services"`
}

type ServiceREDMetric struct {
	ServiceName  string  `json:"serviceName"`
	RequestCount int64   `json:"requestCount"`
	ErrorCount   int64   `json:"errorCount"`
	AvgLatency   float64 `json:"avgLatency"`
	P95Latency   float64 `json:"p95Latency"`
	P99Latency   float64 `json:"p99Latency"`
}

type ServicePerformancePoint struct {
	Timestamp    time.Time `json:"timestamp"    ch:"timestamp"`
	RPS          float64   `json:"rps"          ch:"rps"`
	RequestCount uint64    `json:"requestCount"`
	ErrorCount   uint64    `json:"errorCount"`
	ErrorRate    float64   `json:"errorRate"`
}

type StatusTimeSeriesPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	Status2xx   float64   `json:"status2xx"`
	Status4xx   float64   `json:"status4xx"`
	Status5xx   float64   `json:"status5xx"`
	StatusOther float64   `json:"statusOther"`
}

type LatencyPercentilesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	P50Ms     float64   `json:"p50Ms"`
	P95Ms     float64   `json:"p95Ms"`
	P99Ms     float64   `json:"p99Ms"`
}

type EndpointRatePoint struct {
	Timestamp time.Time `json:"timestamp"`
	HTTPRoute string    `json:"httpRoute"`
	RPS       float64   `json:"rps"`
	ErrorRate *float64  `json:"errorRate"`
	P99Ms     *float64  `json:"p99Ms"`
}

type TopEndpoint struct {
	OperationName string  `json:"operationName"`
	ServiceName   string  `json:"serviceName"`
	SpanKind      string  `json:"spanKind"`
	HTTPRoute     string  `json:"httpRoute"`
	HTTPMethod    string  `json:"httpMethod"`
	RPCSystem     string  `json:"rpcSystem"`
	RPS           float64 `json:"rps"`
	ErrorRate     float64 `json:"errorRate"`
	ErrorCount    int64   `json:"errorCount"`
	TotalCount    int64   `json:"totalCount"`
	P50Ms         float64 `json:"p50Ms"`
	P95Ms         float64 `json:"p95Ms"`
	P99Ms         float64 `json:"p99Ms"`
}

type TopDBQuery struct {
	OperationName string  `json:"operationName"`
	ServiceName   string  `json:"serviceName"`
	DBSystem      string  `json:"dbSystem"`
	RPS           float64 `json:"rps"`
	ErrorRate     float64 `json:"errorRate"`
	ErrorCount    int64   `json:"errorCount"`
	TotalCount    int64   `json:"totalCount"`
	P50Ms         float64 `json:"p50Ms"`
	P95Ms         float64 `json:"p95Ms"`
	P99Ms         float64 `json:"p99Ms"`
}

type redMetricsRow struct {
	ServiceName string    `ch:"service"`
	IsTotal     uint64    `ch:"is_total"`
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
	ServiceName   string  `json:"serviceName"`
	OperationName string  `json:"operationName"`
	P50Ms         float64 `json:"p50Ms"`
	P95Ms         float64 `json:"p95Ms"`
	P99Ms         float64 `json:"p99Ms"`
	SpanCount     int64   `json:"spanCount"`
}

type ServiceSummaryResponse struct {
	ServiceName       string  `json:"serviceName"`
	RequestCount      int64   `json:"requestCount"`
	ErrorCount        int64   `json:"errorCount"`
	RPS               float64 `json:"rps"`
	ErrorRate         float64 `json:"errorRate"`
	P50Ms             float64 `json:"p50Ms"`
	P95Ms             float64 `json:"p95Ms"`
	P99Ms             float64 `json:"p99Ms"`
	CPUUtilization    float64 `json:"cpuUtilization"`
	MemoryUtilization float64 `json:"memoryUtilization"`
	DiskUtilization   float64 `json:"diskUtilization"`
}

type SaturationTimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

type RequestRatePoint struct {
	Timestamp   time.Time `json:"timestamp"    ch:"bucket_at"`
	ServiceName string    `json:"serviceName" ch:"service_name"`
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
