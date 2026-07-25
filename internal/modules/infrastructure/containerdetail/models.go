package containerdetail

import (
	"time"

	"github.com/optikklabs/query/internal/modules/infrastructure/seriesgroup"
)

// SeriesPoint is one display-grain bucket of one named series.
type SeriesPoint = seriesgroup.Point

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

type podMetaRow struct {
	LastSeen     time.Time `ch:"last_seen"`
	Host         string    `ch:"host"`
	Containers   []string  `ch:"containers"`
	Services     []string  `ch:"services"`
	Environments []string  `ch:"environments"`
	Namespaces   []string  `ch:"namespaces"`
	MetricNames  []string  `ch:"metric_names"`
}

type podREDRow struct {
	RequestCount  uint64  `ch:"request_count"`
	ErrorCount    uint64  `ch:"error_count"`
	DurationMsSum float64 `ch:"duration_ms_sum"`
	P95LatencyMs  float32 `ch:"p95_latency_ms"`
}
