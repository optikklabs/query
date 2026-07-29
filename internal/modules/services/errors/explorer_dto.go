package errors

import (
	"time"

	"github.com/optikklabs/query/internal/shared/contracts"
	"github.com/optikklabs/query/internal/shared/spanfilter"
)

// rangeFilters is the body every errors-explorer endpoint shares: a time range
// plus the span filter set, applied to the error spans themselves.
type rangeFilters struct {
	StartTime int64 `json:"startTime"`
	EndTime   int64 `json:"endTime"`

	spanfilter.Filters
}

func (r *rangeFilters) BindTenant(tenantID int64) error {
	r.Filters.TenantID = tenantID
	r.Filters.StartMs = r.StartTime
	r.Filters.EndMs = r.EndTime
	return r.Filters.Validate()
}

type GroupsRequest struct {
	rangeFilters

	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
}

type GroupsResponse struct {
	Results  []ErrorGroup       `json:"results"`
	PageInfo contracts.PageInfo `json:"pageInfo"`
}

type FacetsRequest struct {
	rangeFilters
}

type OverviewRequest struct {
	rangeFilters
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
	TimeBucket time.Time `json:"timeBucket"`
	Errors     int64     `json:"errors"`
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

type rawFacetDimRow struct {
	Dim   string `ch:"dim"`
	Value string `ch:"value"`
	Count uint64 `ch:"cnt"`
}

type rawSummaryRow struct {
	TotalErrors      uint64 `ch:"total_errors"`
	ActiveIssues     uint64 `ch:"active_issues"`
	NewIssues        uint64 `ch:"new_issues"`
	ServicesAffected uint64 `ch:"services_affected"`
}

type rawTrendRow struct {
	TimeBucket time.Time `ch:"time_bucket"`
	Errors     uint64    `ch:"errors"`
}
