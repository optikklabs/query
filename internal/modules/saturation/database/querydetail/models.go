package querydetail

type ServiceCalls struct {
	Service   string `json:"service"`
	CallCount int64  `json:"callCount"`
}

type QuerySummary struct {
	QueryHash      string         `json:"queryHash"`
	QueryText      string         `json:"queryText"`
	DbSystem       string         `json:"dbSystem"`
	CollectionName string         `json:"collectionName"`
	OperationName  string         `json:"operationName"`
	CallCount      int64          `json:"callCount"`
	ErrorCount     int64          `json:"errorCount"`
	P50Ms          *float64       `json:"p50Ms"`
	P95Ms          *float64       `json:"p95Ms"`
	P99Ms          *float64       `json:"p99Ms"`
	AvgMs          float64        `json:"avgMs"`
	TotalTimeMs    float64        `json:"totalTimeMs"`
	AvgRows        *float64       `json:"avgRows"`
	Services       []ServiceCalls `json:"services"`
}

type QueryTimeseriesPoint struct {
	TimeBucket string   `json:"timeBucket"`
	CallCount  int64    `json:"callCount"`
	ErrorCount int64    `json:"errorCount"`
	AvgMs      *float64 `json:"avgMs"`
	P99Ms      *float64 `json:"p99Ms"`
}

type QueryExecution struct {
	Timestamp  string   `json:"timestamp"`
	TraceID    string   `json:"traceId"`
	SpanID     string   `json:"spanId"`
	DurationMs float64  `json:"durationMs"`
	IsError    bool     `json:"isError"`
	Service    string   `json:"service"`
	Host       string   `json:"host"`
	Rows       *float64 `json:"rows"`
}
