package explorer

import (
	"time"

	"github.com/optikklabs/query/internal/modules/traces/filter"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

type QueryRequest struct {
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
	Limit     int    `json:"limit"`
	Cursor    string `json:"cursor"`

	filter.Filters
}

func (r *QueryRequest) BindTenant(tenantID int64) error {
	r.Filters.TenantID = tenantID
	r.Filters.StartMs = r.StartTime
	r.Filters.EndMs = r.EndTime
	return r.Filters.Validate()
}

type QueryResponse struct {
	Results  []Trace  `json:"results"`
	PageInfo PageInfo `json:"pageInfo"`
}

type EnrichRequest struct {
	TraceIDs []string `json:"traceIds"`
}

type TraceEnrichment struct {
	SpanCount  uint32   `json:"spanCount"`
	ErrorCount uint32   `json:"errorCount"`
	HasError   bool     `json:"hasError"`
	ServiceSet []string `json:"serviceSet"`
	StartMs    uint64   `json:"startMs"`
	EndMs      uint64   `json:"endMs"`
	DurationMs float64  `json:"durationMs"`
}

type EnrichResponse struct {
	Enrichments map[string]TraceEnrichment `json:"enrichments"`
}

// traceIndexRowDTO is one root span: only what the root itself knows.
type traceIndexRowDTO struct {
	TraceID        string    `ch:"trace_id"`
	SpanID         string    `ch:"span_id"`
	StartTime      time.Time `ch:"start_time"`
	DurationNs     uint64    `ch:"duration_ns"`
	RootService    string    `ch:"root_service"`
	RootOperation  string    `ch:"root_operation"`
	RootStatus     string    `ch:"root_status"`
	RootHTTPMethod string    `ch:"root_http_method"`
	RootHTTPStatus string    `ch:"root_http_status"`
}

// traceAggRow is the trace-level truth aggregated across all of its spans.
type traceAggRow struct {
	TraceID    string    `ch:"trace_id"`
	SpanCount  uint64    `ch:"span_count"`
	ErrorCount uint64    `ch:"error_count"`
	StartTime  time.Time `ch:"start_time"`
	EndTime    time.Time `ch:"end_time"`
	ServiceSet []string  `ch:"service_set"`
}

type FacetsRequest struct {
	StartTime int64 `json:"startTime"`
	EndTime   int64 `json:"endTime"`

	filter.Filters
}

func (r *FacetsRequest) BindTenant(tenantID int64) error {
	r.Filters.TenantID = tenantID
	r.Filters.StartMs = r.StartTime
	r.Filters.EndMs = r.EndTime
	return r.Filters.Validate()
}

type facetDimRow struct {
	Dim   string `ch:"dim"`
	Value string `ch:"value"`
	Count uint64 `ch:"cnt"`
}

type TrendRequest struct {
	StartTime int64 `json:"startTime"`
	EndTime   int64 `json:"endTime"`

	filter.Filters
}

func (r *TrendRequest) BindTenant(tenantID int64) error {
	r.Filters.TenantID = tenantID
	r.Filters.StartMs = r.StartTime
	r.Filters.EndMs = r.EndTime
	return r.Filters.Validate()
}

// SuggestRequest is a type alias for the shared suggest wire payload.
type SuggestRequest = filterutil.SuggestRequest

// suggestionRow is a type alias for the shared ClickHouse scan target.
type suggestionRow = filterutil.SuggestionRow
