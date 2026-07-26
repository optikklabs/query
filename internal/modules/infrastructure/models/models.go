// Package models holds the infrastructure domain's API DTOs — the wire
// contract shared by the handler and the service. Repository row types are
// deliberately not here: they stay in repository/ and terminate at the
// service, which folds them into these shapes.
package models

import (
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesgroup"
)

// MetricValue is the single-number response used by the CPU and memory
// averages.
type MetricValue struct {
	Value float64 `json:"value"`
}

// CPUInstanceMetric is one instance's folded CPU utilization. A nil Value
// means the instance reported no usable CPU metric in range.
type CPUInstanceMetric struct {
	Host        string   `json:"host"`
	Pod         string   `json:"pod"`
	Container   string   `json:"container"`
	ServiceName string   `json:"serviceName"`
	Value       *float64 `json:"value"`
}

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

// HostStatus is the health classification for a host running a service.
type HostStatus string

const (
	HostHealthy HostStatus = "healthy"
	HostWarn    HostStatus = "warn"
	HostError   HostStatus = "error"
)

const (
	SubsystemKafka    = "kafka"
	SubsystemDatabase = "database"
	SubsystemOther    = "other"
)

type Host struct {
	Host string `json:"host"`

	Subsystem string  `json:"subsystem"`
	CPU       float64 `json:"cpu"`
	Mem       float64 `json:"mem"`
	Disk      float64 `json:"disk"`

	Saturation float64 `json:"saturation"`

	Tone string `json:"tone"`

	Zone         string     `json:"zone,omitempty"`
	RPS          *float64   `json:"rps,omitempty"`
	ErrorRate    *float64   `json:"errorRate,omitempty"`
	P99Ms        *float64   `json:"p99Ms,omitempty"`
	Status       HostStatus `json:"status,omitempty"`
	LastSeen     string     `json:"lastSeen,omitempty"`
	RequestCount int64      `json:"requestCount,omitempty"`
	ErrorCount   int64      `json:"errorCount,omitempty"`
}

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

// SeriesPoint is one display-grain bucket of one named series.
type SeriesPoint = seriesgroup.Point

// HostOverview is the host detail header payload: identity metadata plus
// range-averaged KPI values. Nil KPI fields mean the host does not report
// that metric in the selected range.
type HostOverview struct {
	Host             string     `json:"host"`
	LastSeen         string     `json:"lastSeen,omitempty"`
	Environments     []string   `json:"environments"`
	Namespaces       []string   `json:"namespaces"`
	CPUPct           *float64   `json:"cpuPct"`
	MemoryPct        *float64   `json:"memoryPct"`
	DiskPct          *float64   `json:"diskPct"`
	Load1m           *float64   `json:"load1m"`
	Load5m           *float64   `json:"load5m"`
	Load15m          *float64   `json:"load15m"`
	ProcessCount     *float64   `json:"processCount"`
	AvailableMetrics []string   `json:"availableMetrics"`
	About            *HostAbout `json:"about,omitempty"`
}

// HostAbout is host machine metadata from retained resource attributes.
// Present only when the host reports at least one attribute.
type HostAbout struct {
	OSType        string `json:"osType,omitempty"`
	OSDescription string `json:"osDescription,omitempty"`
	Arch          string `json:"arch,omitempty"`
	HostID        string `json:"hostId,omitempty"`
	CloudProvider string `json:"cloudProvider,omitempty"`
	CloudPlatform string `json:"cloudPlatform,omitempty"`
	CloudRegion   string `json:"cloudRegion,omitempty"`
	CloudZone     string `json:"cloudZone,omitempty"`
	K8SNodeName   string `json:"k8sNodeName,omitempty"`
}

// PodOverview is the container detail header payload: identity metadata plus
// range RED aggregates from span metrics. RequestCount 0 means the pod's
// services reported no traffic in the selected range.
type PodOverview struct {
	Pod              string   `json:"pod"`
	Host             string   `json:"host,omitempty"`
	LastSeen         string   `json:"lastSeen,omitempty"`
	Containers       []string `json:"containers"`
	Services         []string `json:"services"`
	Environments     []string `json:"environments"`
	Namespaces       []string `json:"namespaces"`
	RequestCount     int64    `json:"requestCount"`
	ErrorCount       int64    `json:"errorCount"`
	ErrorRate        float64  `json:"errorRate"`
	AvgLatencyMs     float64  `json:"avgLatencyMs"`
	P95LatencyMs     float64  `json:"p95LatencyMs"`
	AvailableMetrics []string `json:"availableMetrics"`
}
