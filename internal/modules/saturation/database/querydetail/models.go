package querydetail

type ServiceCalls struct {
	Service   string `json:"service" ch:"service"`
	CallCount int64  `json:"call_count" ch:"call_count"`
}

type QuerySummary struct {
	QueryHash      string         `json:"query_hash"`
	QueryText      string         `json:"query_text"`
	DbSystem       string         `json:"db_system"`
	CollectionName string         `json:"collection_name"`
	OperationName  string         `json:"operation_name"`
	CallCount      int64          `json:"call_count"`
	ErrorCount     int64          `json:"error_count"`
	P50Ms          *float64       `json:"p50_ms"`
	P95Ms          *float64       `json:"p95_ms"`
	P99Ms          *float64       `json:"p99_ms"`
	AvgMs          float64        `json:"avg_ms"`
	TotalTimeMs    float64        `json:"total_time_ms"`
	AvgRows        *float64       `json:"avg_rows"`
	Services       []ServiceCalls `json:"services"`
}

type QueryTimeseriesPoint struct {
	TimeBucket string   `json:"time_bucket"`
	CallCount  int64    `json:"call_count"`
	ErrorCount int64    `json:"error_count"`
	AvgMs      *float64 `json:"avg_ms"`
	P99Ms      *float64 `json:"p99_ms"`
}

type QueryExecution struct {
	Timestamp  string   `json:"timestamp"`
	TraceID    string   `json:"trace_id"`
	SpanID     string   `json:"span_id"`
	DurationMs float64  `json:"duration_ms"`
	IsError    bool     `json:"is_error"`
	Service    string   `json:"service"`
	Host       string   `json:"host"`
	Rows       *float64 `json:"rows"`
}
