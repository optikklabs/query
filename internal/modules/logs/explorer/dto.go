package explorer

import (
	"github.com/optikklabs/query/internal/modules/logs/filter"
	"github.com/optikklabs/query/internal/modules/logs/shared/models"
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
	r.Filters.TenantID = tenantID
	r.Filters.StartMs = r.StartTime
	r.Filters.EndMs = r.EndTime
	return r.Filters.Validate()
}

type QueryResponse struct {
	Results  []models.Log    `json:"results"`
	PageInfo models.PageInfo `json:"pageInfo"`
}

// SuggestRequest is a type alias for the shared suggest wire payload.
type SuggestRequest = filterutil.SuggestRequest

// SuggestResponse is a type alias for the shared suggest wire response.
type SuggestResponse = filterutil.SuggestResponse

// Suggestion is a type alias for the shared suggestion value+count pair.
type Suggestion = filterutil.Suggestion

// suggestionRow is a type alias for the shared ClickHouse scan target.
type suggestionRow = filterutil.SuggestionRow
