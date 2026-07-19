package nodes

import "time"

// HTTP response DTOs.

type InfrastructureNode struct {
	Host           string   `json:"host"`
	PodCount       int64    `json:"podCount"`
	ContainerCount int64    `json:"containerCount"`
	Services       []string `json:"services"`
	RequestCount   int64    `json:"requestCount"`
	ErrorCount     int64    `json:"errorCount"`
	ErrorRate      float64  `json:"errorRate"`
	AvgLatencyMs   float64  `json:"avgLatencyMs"`
	P95LatencyMs   float64  `json:"p95LatencyMs"`
	LastSeen       string   `json:"lastSeen"`
}

type InfrastructureNodeService struct {
	ServiceName  string  `json:"serviceName"`
	RequestCount int64   `json:"requestCount"`
	ErrorCount   int64   `json:"errorCount"`
	ErrorRate    float64 `json:"errorRate"`
	AvgLatencyMs float64 `json:"avgLatencyMs"`
	P95LatencyMs float64 `json:"p95LatencyMs"`
	PodCount     int64   `json:"podCount"`
}

type InfrastructureNodeSummary struct {
	HealthyNodes   int64 `json:"healthyNodes"`
	DegradedNodes  int64 `json:"degradedNodes"`
	UnhealthyNodes int64 `json:"unhealthyNodes"`
	TotalPods      int64 `json:"totalPods"`
}

type NodeAggregateRow struct {
	Host          string    `ch:"host"`
	PodCount      uint64    `ch:"pod_count"`
	RequestCount  uint64    `ch:"request_count"`
	ErrorCount    uint64    `ch:"error_count"`
	DurationMsSum float64   `ch:"duration_ms_sum"`
	P95LatencyMs  float32   `ch:"p95_latency_ms"`
	LastSeen      time.Time `ch:"last_seen"`
}

type NodeServiceAggregateRow struct {
	Service       string  `ch:"service"`
	RequestCount  uint64  `ch:"request_count"`
	ErrorCount    uint64  `ch:"error_count"`
	DurationMsSum float64 `ch:"duration_ms_sum"`
	P95LatencyMs  float32 `ch:"p95_latency_ms"`
	PodCount      uint64  `ch:"pod_count"`
}

type NodeSummaryRow struct {
	HealthyNodes   uint64  `ch:"healthy_nodes"`
	DegradedNodes  uint64  `ch:"degraded_nodes"`
	UnhealthyNodes uint64  `ch:"unhealthy_nodes"`
	TotalPods      *uint64 `ch:"total_pods"`
}
