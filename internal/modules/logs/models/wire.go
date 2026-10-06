package models

import (
	"github.com/optikklabs/query/internal/modules/logs/filter"
	"github.com/optikklabs/query/internal/shared/contracts"
	"github.com/optikklabs/query/internal/shared/filterutil"
)

type QueryRequest struct {
	filter.RangeRequest

	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
}

type QueryResponse struct {
	Results  []Log              `json:"results"`
	PageInfo contracts.PageInfo `json:"pageInfo"`
}

type FacetsRequest struct {
	filter.RangeRequest
}

type FacetsResponse struct {
	Facets Facets `json:"facets"`
}

type TrendsRequest struct {
	filter.RangeRequest
}

type SummaryResponse struct {
	Summary Summary `json:"summary"`
}

type TrendResponse struct {
	Trend []TrendBucket `json:"trend"`
}

type SuggestRequest = filterutil.SuggestRequest

type SuggestResponse = filterutil.SuggestResponse

type Suggestion = filterutil.Suggestion
