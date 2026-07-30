package models

import (
	"time"

	"github.com/optikklabs/query/internal/shared/contracts"
)

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

// EndpointRateSeries is the columnar per-endpoint RED time series, carrying
// the top endpoints by volume. Timestamps are unix millis shared by every
// series entry.
type EndpointRateSeries struct {
	Timestamps []int64             `json:"timestamps"`
	Series     []EndpointRateEntry `json:"series"`
	Totals     EndpointRateTotals  `json:"totals"`
}

// EndpointRateTotals covers the whole service, including endpoints outside the
// returned top N. Headline stats read from here so they agree with
// status-timeseries rather than counting only the charted lines.
type EndpointRateTotals struct {
	RPS          []float64  `json:"rps"`
	RequestCount []uint64   `json:"requestCount"`
	ErrorRate    []*float64 `json:"errorRate"`
}

// Nil errorRate/p99Ms entries mean the endpoint had no traffic in that bucket.
type EndpointRateEntry struct {
	OperationName string     `json:"operationName"`
	RPS           []float64  `json:"rps"`
	RequestCount  []uint64   `json:"requestCount"`
	ErrorRate     []*float64 `json:"errorRate"`
	P99Ms         []*float64 `json:"p99Ms"`
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

// RequestRateSeries is the columnar per-service request-rate time series.
// Timestamps are unix millis shared by every series entry.
type RequestRateSeries struct {
	Timestamps []int64            `json:"timestamps"`
	Series     []RequestRateEntry `json:"series"`
}

type RequestRateEntry struct {
	ServiceName string    `json:"serviceName"`
	RPS         []float64 `json:"rps"`
}

type TopEndpointsCursor struct {
	TotalCount    uint64 `json:"cnt"`
	OperationName string `json:"op"`
}

func (c TopEndpointsCursor) IsZero() bool {
	return c.TotalCount == 0 && c.OperationName == ""
}

type PaginatedEndpoints struct {
	Results  []TopEndpoint      `json:"results"`
	PageInfo contracts.PageInfo `json:"pageInfo"`
}

type PaginatedDBQueries struct {
	Results  []TopDBQuery       `json:"results"`
	PageInfo contracts.PageInfo `json:"pageInfo"`
}

type REDMetricsRow struct {
	ServiceName string    `ch:"service_name"`
	IsTotal     uint64    `ch:"is_total"`
	TotalCount  uint64    `ch:"request_total"`
	ErrorCount  uint64    `ch:"error_total"`
	QS          []float64 `ch:"qs"`
	P50Ms       float32   `ch:"p50_ms"`
	P95Ms       float32   `ch:"p95_ms"`
	P99Ms       float32   `ch:"p99_ms"`
}

type OperationBaselineRow struct {
	SpanCount uint64    `ch:"request_total"`
	QS        []float64 `ch:"qs"`
	P50Ms     float32   `ch:"p50_ms"`
	P95Ms     float32   `ch:"p95_ms"`
	P99Ms     float32   `ch:"p99_ms"`
}

type ServiceMetricRow struct {
	Service    string  `ch:"service"`
	MetricName string  `ch:"metric_name"`
	Value      float64 `ch:"value"`
}

type SaturationPointRow struct {
	BucketAt time.Time `ch:"bucket_at"`
	Value    float64   `ch:"value"`
}

type RequestRateRawRow struct {
	BucketAt     time.Time `ch:"bucket_at"`
	RequestCount uint64    `ch:"request_total"`
	ErrorCount   uint64    `ch:"error_total"`
}

type StatusBucketRow struct {
	BucketAt    time.Time `ch:"bucket_at"`
	Status2xx   uint64    `ch:"s2xx"`
	Status4xx   uint64    `ch:"s4xx"`
	Status5xx   uint64    `ch:"s5xx"`
	StatusOther uint64    `ch:"s_other"`
}

type LatencyPercentilesRow struct {
	BucketAt time.Time `ch:"bucket_at"`
	QS       []float64 `ch:"qs"`
	P50Ms    float32   `ch:"p50_ms"`
	P95Ms    float32   `ch:"p95_ms"`
	P99Ms    float32   `ch:"p99_ms"`
}

type EndpointRateRow struct {
	BucketAt      time.Time `ch:"bucket_at"`
	OperationName string    `ch:"operation_name"`
	RequestCount  uint64    `ch:"request_total"`
	ErrorCount    uint64    `ch:"error_total"`
	QS            []float64 `ch:"qs"`
}

type ServiceRequestRateRow struct {
	BucketAt     time.Time `ch:"bucket_at"`
	ServiceName  string    `ch:"service_name"`
	RequestCount uint64    `ch:"request_total"`
}

type TopDBQueryRow struct {
	ServiceName   string    `ch:"service_any"`
	OperationName string    `ch:"operation_name"`
	DBSystem      string    `ch:"db_system_any"`
	TotalCount    uint64    `ch:"request_total"`
	ErrorCount    uint64    `ch:"error_total"`
	QS            []float64 `ch:"qs"`
	P50Ms         float32   `ch:"p50_ms"`
	P95Ms         float32   `ch:"p95_ms"`
	P99Ms         float32   `ch:"p99_ms"`
}

type TopEndpointRow struct {
	ServiceName   string    `ch:"service_any"`
	OperationName string    `ch:"operation_name"`
	SpanKind      string    `ch:"kind_string_any"`
	HTTPRoute     string    `ch:"http_route_any"`
	HTTPMethod    string    `ch:"http_method_any"`
	RPCSystem     string    `ch:"rpc_system_any"`
	TotalCount    uint64    `ch:"request_total"`
	ErrorCount    uint64    `ch:"error_total"`
	QS            []float64 `ch:"qs"`
	P50Ms         float32   `ch:"p50_ms"`
	P95Ms         float32   `ch:"p95_ms"`
	P99Ms         float32   `ch:"p99_ms"`
}
