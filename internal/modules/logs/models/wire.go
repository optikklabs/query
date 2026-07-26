package models

import (
	"github.com/optikklabs/query/internal/modules/logs/filter"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

// QueryRequest is the wire payload for POST /api/v1/logs/query.
type QueryRequest struct {
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
	Limit     int    `json:"limit"`
	Cursor    string `json:"cursor"`

	filter.Filters
}

func (r *QueryRequest) BindTenant(tenantID int64) error {
	return bindTenant(&r.Filters, tenantID, r.StartTime, r.EndTime)
}

type QueryResponse struct {
	Results  []Log    `json:"results"`
	PageInfo PageInfo `json:"pageInfo"`
}

// FacetsRequest is the wire payload for POST /api/v1/logs/facets. Filters are
// embedded directly (no separate compile pass).
type FacetsRequest struct {
	StartTime int64 `json:"startTime"`
	EndTime   int64 `json:"endTime"`

	filter.Filters
}

func (r *FacetsRequest) BindTenant(tenantID int64) error {
	return bindTenant(&r.Filters, tenantID, r.StartTime, r.EndTime)
}

type FacetsResponse struct {
	Facets Facets `json:"facets"`
}

// TrendsRequest is the wire payload shared by POST /api/v1/logs/summary and
// POST /api/v1/logs/trend.
type TrendsRequest struct {
	StartTime int64 `json:"startTime"`
	EndTime   int64 `json:"endTime"`

	filter.Filters
}

func (r *TrendsRequest) BindTenant(tenantID int64) error {
	return bindTenant(&r.Filters, tenantID, r.StartTime, r.EndTime)
}

type SummaryResponse struct {
	Summary Summary `json:"summary"`
}

type TrendResponse struct {
	Trend []TrendBucket `json:"trend"`
}

// bindTenant is the body every request's BindTenant shares: stamp the
// authenticated tenant and lift the wire time range into the filter, then
// validate as one unit.
func bindTenant(f *filter.Filters, tenantID, startTime, endTime int64) error {
	f.TenantID = tenantID
	f.StartMs = startTime
	f.EndMs = endTime
	return f.Validate()
}

// SuggestRequest is a type alias for the shared suggest wire payload.
type SuggestRequest = filterutil.SuggestRequest

// SuggestResponse is a type alias for the shared suggest wire response.
type SuggestResponse = filterutil.SuggestResponse

// Suggestion is a type alias for the shared suggestion value+count pair.
type Suggestion = filterutil.Suggestion
