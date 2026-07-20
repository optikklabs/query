package logfacets //nolint:revive,stylecheck

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

func (r *Request) BindTenant(tenantID int64) error {
	r.Filters.TenantID = tenantID
	r.Filters.StartMs = r.StartTime
	r.Filters.EndMs = r.EndTime
	return r.Filters.Validate()
}

type Response struct {
	Facets models.Facets `json:"facets"`
}
