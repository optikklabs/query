package cloud

import "time"

// ClickHouse row structs (internal). Response DTOs live in dto.go.

// InventoryRow is one provider's inventory aggregate.
type InventoryRow struct {
	Provider  string    `ch:"provider"`
	Accounts  uint64    `ch:"accounts"`
	Regions   uint64    `ch:"regions"`
	Nodes     uint64    `ch:"nodes"`
	Pods      uint64    `ch:"pods"`
	Platforms uint64    `ch:"platforms"`
	Resources uint64    `ch:"resources"`
	LastSeen  time.Time `ch:"last_seen"`
}

// CategoryRow is a per-provider, per-platform entity count.
type CategoryRow struct {
	Provider string `ch:"provider"`
	Platform string `ch:"platform"`
	Count    uint64 `ch:"count"`
}

// HealthRow is a per-provider, per-entity RED aggregate used for health
// classification (same spanmetrics source as the nodes module).
type HealthRow struct {
	Provider     string `ch:"provider"`
	Entity       string `ch:"entity"`
	RequestCount uint64 `ch:"request_count"`
	ErrorCount   uint64 `ch:"error_count"`
}

// RestartRow is the summed latest container-restart count per provider.
type RestartRow struct {
	Provider string `ch:"provider"`
	Restarts uint64 `ch:"restarts"`
}

// AccountRow is a per-account resource breakdown within a provider.
type AccountRow struct {
	Account   string `ch:"account"`
	Resources uint64 `ch:"resources"`
	Nodes     uint64 `ch:"nodes"`
	Pods      uint64 `ch:"pods"`
}

// ResourceRow is one entity needing attention (sorted by error rate).
type ResourceRow struct {
	Entity        string  `ch:"entity"`
	Service       string  `ch:"service"`
	Region        string  `ch:"region"`
	Platform      string  `ch:"platform"`
	RequestCount  uint64  `ch:"request_count"`
	ErrorCount    uint64  `ch:"error_count"`
	DurationMsSum float64 `ch:"duration_ms_sum"`
}
