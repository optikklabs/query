package paths

import (
	"time"
)

type CriticalPathSpan struct {
	SpanID        string  `json:"spanId"        ch:"span_id"`
	OperationName string  `json:"operationName" ch:"operation_name"`
	ServiceName   string  `json:"serviceName"   ch:"service"`
	DurationMs    float64 `json:"durationMs"    ch:"duration_ms"`
}

type ErrorPathSpan struct {
	SpanID        string    `json:"spanId"        ch:"span_id"`
	ParentSpanID  string    `json:"parentSpanId" ch:"parent_span_id"`
	OperationName string    `json:"operationName" ch:"operation_name"`
	ServiceName   string    `json:"serviceName"   ch:"service"`
	Status        string    `json:"status"         ch:"status"`
	StatusMessage string    `json:"statusMessage" ch:"status_message"`
	StartTime     time.Time `json:"startTime"     ch:"start_time"`
	DurationMs    float64   `json:"durationMs"    ch:"duration_ms"`
}
