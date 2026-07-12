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

// ProviderSummary powers the provider hero cards and per-provider tabs.
type ProviderSummary struct {
	Provider   string          `json:"provider"`
	Accounts   int64           `json:"accounts"`
	Regions    int64           `json:"regions"`
	Nodes      int64           `json:"nodes"`
	Pods       int64           `json:"pods"`
	Resources  int64           `json:"resources"`
	Restarts   int64           `json:"restarts"`
	Categories []CategoryCount `json:"categories"`
	Health     HealthCounts    `json:"health"`
	LastSeen   string          `json:"last_seen"`
}

// CloudOverview is the /cloud/overview response.
type CloudOverview struct {
	Providers      []ProviderSummary `json:"providers"`
	TotalResources int64             `json:"total_resources"`
	TotalAccounts  int64             `json:"total_accounts"`
	TotalRegions   int64             `json:"total_regions"`
	TotalNodes     int64             `json:"total_nodes"`
	TotalPods      int64             `json:"total_pods"`
	Unhealthy      int64             `json:"unhealthy"`
	Degraded       int64             `json:"degraded"`
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
	ErrorRate    float64 `json:"error_rate"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	RequestCount int64   `json:"request_count"`
}

// CloudProviderDetail is the /cloud/{provider} response.
type CloudProviderDetail struct {
	Provider  string              `json:"provider"`
	Services  []PlatformService   `json:"services"`
	Accounts  []AccountBreakdown  `json:"accounts"`
	Resources []AttentionResource `json:"resources"`
}
