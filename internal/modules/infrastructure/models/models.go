package models

import (
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesgroup"
)

type MetricValue struct {
	Value float64 `json:"value"`
}

type CPUInstanceMetric struct {
	Host        string   `json:"host"`
	Pod         string   `json:"pod"`
	Container   string   `json:"container"`
	ServiceName string   `json:"serviceName"`
	Value       *float64 `json:"value"`
}

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

type SeriesPoint = seriesgroup.Point

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
