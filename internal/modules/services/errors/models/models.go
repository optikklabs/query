package models

import (
	"time"

	"github.com/optikklabs/query/internal/shared/contracts"
	"github.com/optikklabs/query/internal/shared/spanfilter"
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
	Results  []ErrorGroupTrace  `json:"results"`
	PageInfo contracts.PageInfo `json:"pageInfo"`
}

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
	ServiceName  string  `json:"serviceName"`
	TimestampMs  int64   `json:"timestampMs"`
	RequestCount int64   `json:"requestCount"`
	ErrorCount   int64   `json:"errorCount"`
	ErrorRate    float64 `json:"errorRate"`
	AvgLatency   float64 `json:"avgLatency"`
}

type ErrorHotspotCell struct {
	ServiceName   string `json:"serviceName"   ch:"service"`
	OperationName string `json:"operationName" ch:"operation_name"`
	GroupID       string `json:"groupId"       ch:"error_group_id"`
	ErrorCount    int64  `json:"errorCount"    ch:"error_count"`
}

// RangeFilters is the body every errors-explorer endpoint shares: a time range
// plus the span filter set, applied to the error spans themselves.
type RangeFilters struct {
	StartTime int64 `json:"startTime"`
	EndTime   int64 `json:"endTime"`

	spanfilter.Filters
}

func (r *RangeFilters) BindTenant(tenantID int64) error {
	r.Filters.TenantID = tenantID
	r.Filters.StartMs = r.StartTime
	r.Filters.EndMs = r.EndTime
	return r.Filters.Validate()
}

type GroupsRequest struct {
	RangeFilters

	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
}

type GroupsResponse struct {
	Results  []ErrorGroup       `json:"results"`
	PageInfo contracts.PageInfo `json:"pageInfo"`
}

type FacetsRequest struct {
	RangeFilters
}

type OverviewRequest struct {
	RangeFilters
}

// OverviewResponse drives the KPI strip and the error-volume chart from one
// round trip; both read the same filtered span set.
type OverviewResponse struct {
	Summary Summary       `json:"summary"`
	Trend   []TrendBucket `json:"trend"`
}

type Summary struct {
	TotalErrors      int64 `json:"totalErrors"`
	ActiveIssues     int64 `json:"activeIssues"`
	NewIssues        int64 `json:"newIssues"`
	ServicesAffected int64 `json:"servicesAffected"`
}

type TrendBucket struct {
	TimeBucketMs int64 `json:"timeBucketMs"`
	Errors       int64 `json:"errors"`
}

type FacetBucket struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// Facets keys mirror the DSL field names the search bar emits, so a facet
// click and a typed filter produce the same query.
type Facets struct {
	Service       []FacetBucket `json:"service,omitempty"`
	Operation     []FacetBucket `json:"operation,omitempty"`
	HTTPStatus    []FacetBucket `json:"httpStatus,omitempty"`
	ExceptionType []FacetBucket `json:"exceptionType,omitempty"`
}

type RawServiceRateRow struct {
	ServiceName   string    `ch:"service_name"`
	BucketAt      time.Time `ch:"bucket_at"`
	RequestCount  uint64    `ch:"request_total"`
	ErrorCount    uint64    `ch:"error_total"`
	DurationMsSum float64   `ch:"duration_ms_total"`
}

type RawErrorGroupRow struct {
	GroupID          string    `ch:"error_group_id"`
	ServiceName      string    `ch:"service"`
	OperationName    string    `ch:"operation_name"`
	HTTPStatusBucket string    `ch:"http_status_bucket"`
	ErrorCount       uint64    `ch:"error_count"`
	LastOccurrence   time.Time `ch:"last_occurrence"`
	FirstOccurrence  time.Time `ch:"first_occurrence"`
	StatusMessage    string    `ch:"error_message"`
	SampleTraceID    string    `ch:"sample_trace_id"`
}

type RawErrorGroupDetailRow struct {
	GroupID         string    `ch:"error_group_id"`
	ServiceName     string    `ch:"service"`
	OperationName   string    `ch:"operation_name"`
	HTTPStatusCode  uint16    `ch:"http_status_code"`
	ErrorCount      uint64    `ch:"error_count"`
	LastOccurrence  time.Time `ch:"last_occurrence"`
	FirstOccurrence time.Time `ch:"first_occurrence"`
	ExceptionType   string    `ch:"exception_type"`
}

type RawErrorGroupTraceRow struct {
	TraceID    string    `ch:"trace_id"`
	SpanID     string    `ch:"span_id"`
	Timestamp  time.Time `ch:"timestamp"`
	DurationMs float64   `ch:"duration_ms"`
	StatusCode string    `ch:"status_code"`
}

type RawErrorLatestOccurrenceRow struct {
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

type RawTimeBucketCountRow struct {
	BucketAt time.Time `ch:"bucket_at"`
	Count    uint64    `ch:"count"`
}

type RawErrorHotspotRow struct {
	ServiceName   string `ch:"service"`
	OperationName string `ch:"operation_name"`
	GroupID       string `ch:"error_group_id"`
	ErrorCount    uint64 `ch:"error_count"`
}

type RawFacetDimRow struct {
	Dim   string `ch:"dim"`
	Value string `ch:"value"`
	Count uint64 `ch:"cnt"`
}

type RawSummaryRow struct {
	TotalErrors      uint64 `ch:"total_errors"`
	ActiveIssues     uint64 `ch:"active_issues"`
	NewIssues        uint64 `ch:"new_issues"`
	ServicesAffected uint64 `ch:"services_affected"`
}

type RawTrendRow struct {
	TimeBucket time.Time `ch:"time_bucket"`
	Errors     uint64    `ch:"errors"`
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
