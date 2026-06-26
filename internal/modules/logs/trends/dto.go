package log_trends //nolint:revive,stylecheck

import (
	"github.com/optikklabs/query/internal/modules/logs/filter"
	"github.com/optikklabs/query/internal/modules/logs/shared/models"
)

// Request is the wire payload shared by POST /api/v1/logs/summary and
// POST /api/v1/logs/trend.
type Request struct {
	StartTime int64 `json:"startTime"`
	EndTime   int64 `json:"endTime"`

	filter.Filters
}

type SummaryResponse struct {
	Summary models.Summary `json:"summary"`
}

type TrendResponse struct {
	Trend []models.TrendBucket `json:"trend"`
}
