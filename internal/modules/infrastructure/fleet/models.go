package fleet

import "time"

// FleetPod aggregates root-span traffic by Kubernetes pod name and host.
type FleetPod struct {
	PodName      string   `json:"podName"`
	Host         string   `json:"host"`
	Services     []string `json:"services"`
	RequestCount int64    `json:"requestCount"`
	ErrorCount   int64    `json:"errorCount"`
	ErrorRate    float64  `json:"errorRate"`
	AvgLatencyMs float64  `json:"avgLatencyMs"`
	P95LatencyMs float64  `json:"p95LatencyMs"`
	LastSeen     string   `json:"lastSeen"`
}

type FleetPodAggregateRow struct {
	Pod           string    `ch:"pod"`
	Host          string    `ch:"host"`
	Services      []string  `ch:"services"`
	RequestCount  uint64    `ch:"request_total"`
	ErrorCount    uint64    `ch:"error_total"`
	DurationMsSum float64   `ch:"duration_ms_total"`
	P95LatencyMs  float32   `ch:"p95_latency_ms"`
	LastSeen      time.Time `ch:"last_seen"`
}
