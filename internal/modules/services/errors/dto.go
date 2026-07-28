package errors

import "time"

type rawServiceRateRow struct {
	ServiceName   string    `ch:"service_name"`
	BucketAt      time.Time `ch:"bucket_at"`
	RequestCount  uint64    `ch:"request_total"`
	ErrorCount    uint64    `ch:"error_total"`
	DurationMsSum float64   `ch:"duration_ms_total"`
}

type rawServiceErrorRow struct {
	ServiceName string    `ch:"service_name"`
	BucketAt    time.Time `ch:"bucket_at"`
	ErrorCount  uint64    `ch:"error_total"`
}

type rawErrorGroupRow struct {
	GroupID          string    `ch:"error_group_id"`
	ServiceName      string    `ch:"service"`
	OperationName    string    `ch:"operation_name"`
	HTTPStatusBucket string    `ch:"http_status_bucket"`
	ErrorCount       uint64    `ch:"error_count"`
	LastOccurrence   time.Time `ch:"last_occurrence"`
	FirstOccurrence  time.Time `ch:"first_occurrence"`
	StatusMessage    string    `ch:"status_message"`
	SampleTraceID    string    `ch:"sample_trace_id"`
}

type rawErrorGroupDetailRow struct {
	GroupID         string    `ch:"error_group_id"`
	ServiceName     string    `ch:"service"`
	OperationName   string    `ch:"operation_name"`
	HTTPStatusCode  uint16    `ch:"http_status_code"`
	ErrorCount      uint64    `ch:"error_count"`
	LastOccurrence  time.Time `ch:"last_occurrence"`
	FirstOccurrence time.Time `ch:"first_occurrence"`
	ExceptionType   string    `ch:"exception_type"`
}

type rawErrorGroupTraceRow struct {
	TraceID    string    `ch:"trace_id"`
	SpanID     string    `ch:"span_id"`
	Timestamp  time.Time `ch:"timestamp"`
	DurationMs float64   `ch:"duration_ms"`
	StatusCode string    `ch:"status_code"`
}

type rawErrorLatestOccurrenceRow struct {
	TraceID          string    `ch:"trace_id"`
	SpanID           string    `ch:"span_id"`
	Timestamp        time.Time `ch:"timestamp"`
	DurationMs       float64   `ch:"duration_ms"`
	ExceptionMessage string    `ch:"exception_message"`
	StackTrace       string    `ch:"exception_stacktrace"`
	HTTPMethod       string    `ch:"http_method"`
	HTTPRoute        string    `ch:"http_route"`
	HTTPStatusCode   string    `ch:"response_status_code"`
	ServiceVersion   string    `ch:"service_version"`
	Environment      string    `ch:"environment"`
	Pod              string    `ch:"pod"`
	Host             string    `ch:"host"`
}

type rawTimeBucketCountRow struct {
	BucketAt time.Time `ch:"bucket_at"`
	Count    uint64    `ch:"count"`
}
type rawErrorHotspotRow struct {
	ServiceName   string `ch:"service"`
	OperationName string `ch:"operation_name"`
	GroupID       string `ch:"error_group_id"`
	ErrorCount    uint64 `ch:"error_count"`
}

type ErrorGroupsCursor struct {
	ErrorCount uint64 `json:"cnt"`
	GroupID    string `json:"id"`
}

func (c ErrorGroupsCursor) IsZero() bool {
	return c.ErrorCount == 0 && c.GroupID == ""
}

type ErrorTracesCursor struct {
	Timestamp time.Time `json:"ts"`
	SpanID    string    `json:"sid"`
}

func (c ErrorTracesCursor) IsZero() bool {
	return c.Timestamp.IsZero() && c.SpanID == ""
}

type PageInfo struct {
	HasMore    bool   `json:"hasMore"`
	NextCursor string `json:"nextCursor,omitempty"`
	Limit      int    `json:"limit"`
}

type PaginatedErrorGroups struct {
	Results  []ErrorGroup `json:"results"`
	PageInfo PageInfo     `json:"pageInfo"`
}
