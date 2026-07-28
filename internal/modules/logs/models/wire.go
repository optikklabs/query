package models

import (
	"github.com/optikklabs/query/internal/modules/logs/filter"
	"github.com/optikklabs/query/internal/shared/contracts"
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
	return bindTenant(&r.Filters, tenantID, r.StartTime, r.EndTime)
}

type QueryResponse struct {
	Results  []Log    `json:"results"`
	PageInfo contracts.PageInfo `json:"pageInfo"`
}

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

func bindTenant(f *filter.Filters, tenantID, startTime, endTime int64) error {
	f.TenantID = tenantID
	f.StartMs = startTime
	f.EndMs = endTime
	return f.Validate()
}

type SuggestRequest = filterutil.SuggestRequest

type SuggestResponse = filterutil.SuggestResponse

type Suggestion = filterutil.Suggestion
