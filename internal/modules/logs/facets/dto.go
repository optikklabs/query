package log_facets //nolint:revive,stylecheck

import (
	"github.com/optikklabs/query/internal/modules/logs/filter"
	"github.com/optikklabs/query/internal/modules/logs/shared/models"
)

// Request is the wire payload for POST /api/v1/logs/facets. Filters are
// embedded directly (no separate compile pass).
type Request struct {
	StartTime int64 `json:"startTime"`
	EndTime   int64 `json:"endTime"`

	filter.Filters
}

type Response struct {
	Facets models.Facets `json:"facets"`
}
