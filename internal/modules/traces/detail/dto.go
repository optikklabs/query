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
