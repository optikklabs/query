package errors

import (
	"time"
)

type ErrorGroup struct {
	GroupID         string    `json:"groupId"`
	ServiceName     string    `json:"serviceName"`
	OperationName   string    `json:"operationName"`
	StatusMessage   string    `json:"statusMessage"`
	HTTPStatusCode  int       `json:"httpStatusCode"`
	ErrorCount      int64     `json:"errorCount"`
	LastOccurrence  time.Time `json:"lastOccurrence"`
	FirstOccurrence time.Time `json:"firstOccurrence"`
	SampleTraceID   string    `json:"sampleTraceId"`
}

type ErrorGroupDetail struct {
	GroupID         string    `json:"groupId"`
	ServiceName     string    `json:"serviceName"`
	OperationName   string    `json:"operationName"`
	HTTPStatusCode  int       `json:"httpStatusCode"`
	ErrorCount      int64     `json:"errorCount"`
	LastOccurrence  time.Time `json:"lastOccurrence"`
	FirstOccurrence time.Time `json:"firstOccurrence"`
	ExceptionType   string    `json:"exceptionType,omitempty"`
}

type ErrorGroupTrace struct {
	TraceID    string    `json:"traceId"`
	SpanID     string    `json:"spanId"`
	Timestamp  time.Time `json:"timestamp"`
	DurationMs float64   `json:"durationMs"`
	StatusCode string    `json:"statusCode"`
}

type PaginatedErrorTraces struct {
	Results  []ErrorGroupTrace `json:"results"`
	PageInfo PageInfo          `json:"pageInfo"`
}

// ErrorLatestOccurrence is the context of a group's most recent error span.
type ErrorLatestOccurrence struct {
	TraceID        string    `json:"traceId"`
	SpanID         string    `json:"spanId"`
	Timestamp      time.Time `json:"timestamp"`
	DurationMs     float64   `json:"durationMs"`
	Message        string    `json:"message"`
	Stacktrace     string    `json:"stacktrace,omitempty"`
	HTTPMethod     string    `json:"httpMethod"`
	HTTPRoute      string    `json:"httpRoute"`
	HTTPStatusCode string    `json:"httpStatusCode"`
	ServiceVersion string    `json:"serviceVersion"`
	Environment    string    `json:"environment"`
	Pod            string    `json:"pod"`
	Host           string    `json:"host"`
}

type ErrorFacet struct {
	Name  string  `json:"name"`
	Count int64   `json:"count"`
	Pct   float64 `json:"pct"`
}

type ErrorFacetGroup struct {
	Key    string       `json:"key"`
	Facets []ErrorFacet `json:"facets"`
}

type TimeSeriesPoint struct {
	ServiceName  string    `json:"serviceName"`
	Timestamp    time.Time `json:"timestamp"`
	RequestCount int64     `json:"requestCount"`
	ErrorCount   int64     `json:"errorCount"`
	ErrorRate    float64   `json:"errorRate"`
	AvgLatency   float64   `json:"avgLatency"`
}

type ErrorHotspotCell struct {
	ServiceName   string `json:"serviceName"   ch:"service"`
	OperationName string `json:"operationName" ch:"operation_name"`
	GroupID       string `json:"groupId"       ch:"error_group_id"`
	ErrorCount    int64  `json:"errorCount"    ch:"error_count"`
}
