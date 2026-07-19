package detail

import (
	"time"
)

type TraceSummary struct {
	TraceID        string   `json:"traceId"`
	StartMs        uint64   `json:"startMs"`
	EndMs          uint64   `json:"endMs"`
	DurationMs     float64  `json:"durationMs"`
	RootService    string   `json:"rootService"`
	RootOperation  string   `json:"rootOperation"`
	RootStatus     string   `json:"rootStatus,omitempty"`
	RootHTTPMethod string   `json:"rootHttpMethod,omitempty"`
	RootHTTPStatus string   `json:"rootHttpStatus,omitempty"`
	SpanCount      uint32   `json:"spanCount"`
	HasError       bool     `json:"hasError"`
	ErrorCount     uint32   `json:"errorCount"`
	ServiceSet     []string `json:"serviceSet,omitempty"`
	Truncated      bool     `json:"truncated,omitempty"`
	// RootMissing marks a trace whose root span was never ingested. The summary
	// then describes the earliest span rather than the true entry point.
	RootMissing bool `json:"rootMissing,omitempty"`
}

type SpanEvent struct {
	SpanID     string    `json:"spanId"     ch:"span_id"`
	TraceID    string    `json:"traceId"    ch:"trace_id"`
	EventName  string    `json:"eventName"  ch:"event_name"`
	Timestamp  time.Time `json:"timestamp"   ch:"timestamp"`
	Attributes string    `json:"attributes"`
}

type SpanAttributes struct {
	SpanID                string            `json:"spanId"`
	TraceID               string            `json:"traceId"`
	OperationName         string            `json:"operationName"`
	ServiceName           string            `json:"serviceName"`
	AttributesString      map[string]string `json:"attributesString"`
	ResourceAttrs         map[string]string `json:"resourceAttributes"`
	ExceptionType         string            `json:"exceptionType,omitempty"`
	ExceptionMessage      string            `json:"exceptionMessage,omitempty"`
	ExceptionStacktrace   string            `json:"exceptionStacktrace,omitempty"`
	DBSystem              string            `json:"dbSystem,omitempty"`
	DBName                string            `json:"dbName,omitempty"`
	DBStatement           string            `json:"dbStatement,omitempty"`
	DBStatementNormalized string            `json:"dbStatementNormalized,omitempty"`
	Attributes            map[string]string `json:"attributes,omitempty"`
	Links                 []SpanLink        `json:"links,omitempty"`
}

type SpanLink struct {
	TraceID    string            `json:"traceId"`
	SpanID     string            `json:"spanId"`
	TraceState string            `json:"traceState,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type RelatedTrace struct {
	TraceID       string    `json:"traceId"       ch:"trace_id"`
	SpanID        string    `json:"spanId"        ch:"span_id"`
	OperationName string    `json:"operationName" ch:"operation_name"`
	ServiceName   string    `json:"serviceName"   ch:"service"`
	DurationMs    float64   `json:"durationMs"    ch:"duration_ms"`
	Status        string    `json:"status"         ch:"status"`
	StartTime     time.Time `json:"startTime"     ch:"start_time"`
}

type SpanListItem struct {
	SpanID        string    `json:"spanId"        ch:"span_id"`
	ParentSpanID  string    `json:"parentSpanId" ch:"parent_span_id"`
	TraceID       string    `json:"traceId"       ch:"trace_id"`
	ServiceName   string    `json:"serviceName"   ch:"service"`
	OperationName string    `json:"operationName" ch:"name"`
	KindString    string    `json:"spanKind"      ch:"kind_string"`
	StatusCode    string    `json:"status"         ch:"status_code_string"`
	HasError      bool      `json:"hasError"      ch:"has_error"`
	DurationMs    float64   `json:"durationMs"    ch:"duration_ms"`
	Timestamp     time.Time `json:"-"              ch:"timestamp"`
	StartNs       int64     `json:"startNs"       ch:"-"`
}
