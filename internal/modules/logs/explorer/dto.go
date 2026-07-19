package explorer

import (
	"github.com/optikklabs/query/internal/modules/logs/filter"
	"github.com/optikklabs/query/internal/modules/logs/shared/models"
)

// QueryRequest is the wire payload for POST /api/v1/logs/query.
type QueryRequest struct {
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
	Limit     int    `json:"limit"`
	Cursor    string `json:"cursor"`

	filter.Filters
}

type QueryResponse struct {
	Results  []models.Log    `json:"results"`
	PageInfo models.PageInfo `json:"pageInfo"`
}

// SuggestRequest is the wire payload for POST /api/v1/logs/suggest. It
// mirrors the traces suggest contract.
type SuggestRequest struct {
	StartTime int64  `json:"startTime"`
	EndTime   int64  `json:"endTime"`
	Field     string `json:"field"`
	Prefix    string `json:"prefix"`
	Limit     int    `json:"limit"`
}

type SuggestResponse struct {
	Suggestions []Suggestion `json:"suggestions"`
}

type Suggestion struct {
	Value string `json:"value"`
	Count uint64 `json:"count"`
}

type suggestionRow struct {
	Value string `ch:"value"`
	Count uint64 `ch:"count"`
}
