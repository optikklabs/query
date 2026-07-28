package cloud

type CategoryCount struct {
	Category string `json:"category"`
	Count    int64  `json:"count"`
}

type HealthCounts struct {
	Healthy   int64 `json:"healthy"`
	Degraded  int64 `json:"degraded"`
	Unhealthy int64 `json:"unhealthy"`
}

type PlatformService struct {
	Platform string `json:"platform"`
	Category string `json:"category"`
	Count    int64  `json:"count"`
}

type AccountBreakdown struct {
	Account   string `json:"account"`
	Resources int64  `json:"resources"`
	Nodes     int64  `json:"nodes"`
	Pods      int64  `json:"pods"`
}

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
