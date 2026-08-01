package explorer

import (
	"time"

	"github.com/optikklabs/query/internal/shared/contracts"
	"github.com/optikklabs/query/internal/shared/filterutil"
	"github.com/optikklabs/query/internal/shared/spanfilter"
)

type QueryRequest struct {
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
	Limit     int    `json:"limit"`
	Cursor    string `json:"cursor"`

	spanfilter.Filters
}

func (r *QueryRequest) BindTenant(tenantID int64) error {
	r.Filters.TenantID = tenantID
	r.Filters.StartMs = r.StartTime
	r.Filters.EndMs = r.EndTime
	return r.Filters.Validate()
}

type QueryResponse struct {
	Results  []Trace            `json:"results"`
	PageInfo contracts.PageInfo `json:"pageInfo"`
}

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
	RootEndpoint   string    `ch:"root_endpoint"`
	Environment    string    `ch:"environment"`
}

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

	spanfilter.Filters
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

type trendRow struct {
	TimeBucket time.Time `ch:"time_bucket"`
	Total      uint64    `ch:"total"`
	Errors     uint64    `ch:"errors"`
}

type TrendRequest struct {
	StartTime int64 `json:"startTime"`
	EndTime   int64 `json:"endTime"`

	spanfilter.Filters
}

func (r *TrendRequest) BindTenant(tenantID int64) error {
	r.Filters.TenantID = tenantID
	r.Filters.StartMs = r.StartTime
	r.Filters.EndMs = r.EndTime
	return r.Filters.Validate()
}

type SuggestRequest = filterutil.SuggestRequest

type suggestionRow = filterutil.SuggestionRow
