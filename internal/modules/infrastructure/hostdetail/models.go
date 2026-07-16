package hostdetail

import "time"

// SeriesPoint is one display-grain bucket of one named series.
type SeriesPoint struct {
	TimeBucket time.Time `json:"time_bucket" ch:"time_bucket"`
	Series     string    `json:"series"      ch:"series"`
	Value      float64   `json:"value"       ch:"value"`
}

// HostOverview is the host detail header payload: identity metadata plus
// range-averaged KPI values. Nil KPI fields mean the host does not report
// that metric in the selected range.
type HostOverview struct {
	Host             string     `json:"host"`
	LastSeen         string     `json:"last_seen,omitempty"`
	Environments     []string   `json:"environments"`
	Namespaces       []string   `json:"namespaces"`
	CPUPct           *float64   `json:"cpu_pct"`
	MemoryPct        *float64   `json:"memory_pct"`
	DiskPct          *float64   `json:"disk_pct"`
	Load1m           *float64   `json:"load_1m"`
	Load5m           *float64   `json:"load_5m"`
	Load15m          *float64   `json:"load_15m"`
	ProcessCount     *float64   `json:"process_count"`
	AvailableMetrics []string   `json:"available_metrics"`
	About            *HostAbout `json:"about,omitempty"`
}

// HostAbout is host machine metadata from retained resource attributes.
// Present only when the host reports at least one attribute.
type HostAbout struct {
	OSType        string `json:"os_type,omitempty"`
	OSDescription string `json:"os_description,omitempty"`
	Arch          string `json:"arch,omitempty"`
	HostID        string `json:"host_id,omitempty"`
	CloudProvider string `json:"cloud_provider,omitempty"`
	CloudPlatform string `json:"cloud_platform,omitempty"`
	CloudRegion   string `json:"cloud_region,omitempty"`
	CloudZone     string `json:"cloud_zone,omitempty"`
	K8SNodeName   string `json:"k8s_node_name,omitempty"`
}

type hostMetaRow struct {
	LastSeen      time.Time `ch:"last_seen"`
	Environments  []string  `ch:"environments"`
	Namespaces    []string  `ch:"namespaces"`
	MetricNames   []string  `ch:"metric_names"`
	OSType        string    `ch:"os_type"`
	OSDescription string    `ch:"os_description"`
	HostArch      string    `ch:"host_arch"`
	HostID        string    `ch:"host_id"`
	CloudProvider string    `ch:"cloud_provider"`
	CloudPlatform string    `ch:"cloud_platform"`
	CloudRegion   string    `ch:"cloud_region"`
	CloudZone     string    `ch:"cloud_zone"`
	K8SNodeName   string    `ch:"k8s_node_name"`
}

type kpiRow struct {
	MetricName string  `ch:"metric_name"`
	State      string  `ch:"state"`
	Mount      string  `ch:"mount"`
	Value      float64 `ch:"value"`
}
