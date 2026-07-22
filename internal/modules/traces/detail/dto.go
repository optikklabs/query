package detail

import (
	"time"
)

type spanEventTuple struct {
	Name         string            `ch:"name"`
	TimeUnixNano uint64            `ch:"time_unix_nano"`
	Attributes   map[string]string `ch:"attributes"`
}

type spanLinkTuple struct {
	TraceID    string            `ch:"trace_id"`
	SpanID     string            `ch:"span_id"`
	TraceState string            `ch:"trace_state"`
	Attributes map[string]string `ch:"attributes"`
}

type exceptionRow struct {
	SpanID              string    `ch:"span_id"`
	TraceID             string    `ch:"trace_id"`
	Timestamp           time.Time `ch:"timestamp"`
	ExceptionType       string    `ch:"exception_type"`
	ExceptionMessage    string    `ch:"exception_message"`
	ExceptionStacktrace string    `ch:"exception_stacktrace"`
}

type spanEventCombinedRow struct {
	SpanID              string           `ch:"span_id"`
	TraceID             string           `ch:"trace_id"`
	Timestamp           time.Time        `ch:"timestamp"`
	Events              []spanEventTuple `ch:"events"`
	ExceptionType       string           `ch:"exception_type"`
	ExceptionMessage    string           `ch:"exception_message"`
	ExceptionStacktrace string           `ch:"exception_stacktrace"`
}

type spanAttributeRow struct {
	SpanID              string            `ch:"span_id"`
	TraceID             string            `ch:"trace_id"`
	OperationName       string            `ch:"operation_name"`
	ServiceName         string            `ch:"service"`
	Attributes          map[string]string `ch:"attributes"`
	ExceptionType       string            `ch:"exception_type"`
	ExceptionMessage    string            `ch:"exception_message"`
	ExceptionStacktrace string            `ch:"exception_stacktrace"`
	DBSystem            string            `ch:"db_system"`
	DBName              string            `ch:"db_name"`
	DBStatement         string            `ch:"db_statement"`
	Links               []spanLinkTuple   `ch:"links"`
}

type traceSummaryRow struct {
	TraceID        string    `ch:"trace_id"`
	StartTime      time.Time `ch:"start_time"`
	EndTime        time.Time `ch:"end_time"`
	RootService    string    `ch:"root_service"`
	RootOperation  string    `ch:"root_operation"`
	RootStatus     string    `ch:"root_status"`
	RootHTTPMethod string    `ch:"root_http_method"`
	RootHTTPStatus string    `ch:"root_http_status"`
	SpanCount      uint64    `ch:"span_count"`
	ErrorCount     uint64    `ch:"error_count"`
	HasError       bool      `ch:"trace_has_error"`
	ServiceSet     []string  `ch:"service_set"`
	RootMissing    bool      `ch:"root_missing"`
}
