package cloud

import "time"

type InventoryRow struct {
	Provider  string    `ch:"provider"  json:"provider"`
	Accounts  uint64    `ch:"accounts"  json:"accounts"`
	Regions   uint64    `ch:"regions"   json:"regions"`
	Nodes     uint64    `ch:"nodes"     json:"nodes"`
	Pods      uint64    `ch:"pods"      json:"pods"`
	Platforms uint64    `ch:"platforms" json:"platforms"`
	Resources uint64    `ch:"resources" json:"resources"`
	LastSeen  time.Time `ch:"last_seen" json:"lastSeen"`
}

type CategoryRow struct {
	Provider string `ch:"provider"`
	Platform string `ch:"platform"`
	Count    uint64 `ch:"count"`
}

type HealthRow struct {
	Provider     string `ch:"provider"`
	Entity       string `ch:"entity"`
	RequestCount uint64 `ch:"request_total"`
	ErrorCount   uint64 `ch:"error_total"`
}

type RestartRow struct {
	Provider string `ch:"provider"`
	Restarts uint64 `ch:"restarts"`
}

type AccountRow struct {
	Account   string `ch:"account"`
	Resources uint64 `ch:"resources"`
	Nodes     uint64 `ch:"nodes"`
	Pods      uint64 `ch:"pods"`
}

type ResourceRow struct {
	Entity        string  `ch:"entity"`
	Service       string  `ch:"service_any"`
	Region        string  `ch:"region"`
	Platform      string  `ch:"platform"`
	RequestCount  uint64  `ch:"request_total"`
	ErrorCount    uint64  `ch:"error_total"`
	DurationMsSum float64 `ch:"duration_ms_total"`
}
