package cloud

// HTTP response DTOs — the API contract, independent of ClickHouse rows.

// CategoryCount is one category bucket total for a provider.
type CategoryCount struct {
	Category string `json:"category"`
	Count    int64  `json:"count"`
}

// HealthCounts holds the entity health classification for a provider.
type HealthCounts struct {
	Healthy   int64 `json:"healthy"`
	Degraded  int64 `json:"degraded"`
	Unhealthy int64 `json:"unhealthy"`
}

// PlatformService is one platform tile in the per-provider service grid.
type PlatformService struct {
	Platform string `json:"platform"`
	Category string `json:"category"`
	Count    int64  `json:"count"`
}

// AccountBreakdown is one account/project row in the provider detail.
type AccountBreakdown struct {
	Account   string `json:"account"`
	Resources int64  `json:"resources"`
	Nodes     int64  `json:"nodes"`
	Pods      int64  `json:"pods"`
}

// AttentionResource is one entity needing attention.
type AttentionResource struct {
	Entity       string  `json:"entity"`
	Service      string  `json:"service"`
	Region       string  `json:"region"`
	Platform     string  `json:"platform"`
	Health       string  `json:"health"`
	ErrorRate    float64 `json:"errorRate"`
	AvgLatencyMs float64 `json:"avgLatencyMs"`
	RequestCount int64   `json:"requestCount"`
}
