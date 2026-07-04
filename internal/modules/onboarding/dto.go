package onboarding

import "time"

// StatusResponse powers GET /api/v1/onboarding/status for the CLI wizard.
type StatusResponse struct {
	Provisioned   bool       `json:"provisioned"`
	Status        string     `json:"status"`
	Slug          string     `json:"slug"`
	APIKey        string     `json:"api_key"`
	FirstSpanAt   *time.Time `json:"first_span_at"`
	FirstLogAt    *time.Time `json:"first_log_at"`
	FirstMetricAt *time.Time `json:"first_metric_at"`
}
