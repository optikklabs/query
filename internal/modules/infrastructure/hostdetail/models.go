package hostdetail

import "time"

// SeriesPoint is one display-grain bucket of one named series.
type SeriesPoint struct {
	TimeBucket time.Time `json:"timeBucket" ch:"time_bucket"`
	Series     string    `json:"series"      ch:"series"`
	Value      float64   `json:"value"       ch:"value"`
}

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
