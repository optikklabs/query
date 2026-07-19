package hosts

import (
	"strings"
	"time"
)

// HostStatus is the health classification for a host running a service.
type HostStatus string

const (
	HostHealthy HostStatus = "healthy"
	HostWarn    HostStatus = "warn"
	HostError   HostStatus = "error"
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

type hostMetricRow struct {
	Host       string  `ch:"host"`
	MetricName string  `ch:"metric_name"`
	Value      float64 `ch:"value"`
}

type hostSpansRow struct {
	Host         string    `ch:"host"`
	Zone         string    `ch:"zone"`
	RequestCount uint64    `ch:"request_count"`
	ErrorCount   uint64    `ch:"error_count"`
	P99Ms        float32   `ch:"p99_ms"`
	LastSeen     time.Time `ch:"last_seen"`
}

const (
	SubsystemKafka    = "kafka"
	SubsystemDatabase = "database"
	SubsystemOther    = "other"
)

func subsystemForHost(host string) string {
	h := strings.ToLower(host)
	switch {
	case strings.HasPrefix(h, "kafka") || strings.Contains(h, "broker"):
		return SubsystemKafka
	case strings.HasPrefix(h, "pg") || strings.HasPrefix(h, "postgres") || strings.HasPrefix(h, "mysql") || strings.HasPrefix(h, "db"):
		return SubsystemDatabase
	default:
		return SubsystemOther
	}
}

func toneForSaturation(pct float64) string {
	switch {
	case pct >= 90:
		return "err"
	case pct >= 70:
		return "warn"
	default:
		return "ok"
	}
}
