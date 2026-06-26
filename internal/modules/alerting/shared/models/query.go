package models

// MonitorQuery is a discriminated union over supported monitor types.
// Dispatches to the corresponding shared/query implementation.
type MonitorQuery struct {
	Metric *MetricQuery `json:"metric,omitempty"`
	APM    *APMQuery    `json:"apm,omitempty"`
	Log    *LogQuery    `json:"log,omitempty"`
}

type MetricQuery struct {
	Metric string `json:"metric"`

	Aggregation string `json:"aggregation"`

	WindowSec int `json:"window_sec"`
}

type APMQuery struct {
	Service  string `json:"service"`
	Resource string `json:"resource,omitempty"`

	Track     string `json:"track"`
	WindowSec int    `json:"window_sec"`
}

type LogQuery struct {
	Query string `json:"query"`

	GroupBy   string `json:"group_by,omitempty"`
	WindowSec int    `json:"window_sec"`
}

type NotifyTargets struct {
	ChannelIDs []int64 `json:"channel_ids"`
}

var SupportedMonitorTypes = []string{"metric", "apm", "log"}

var SupportedPriorities = []string{"P1", "P2", "P3", "P4"}

func IsValidType(t string) bool {
	for _, v := range SupportedMonitorTypes {
		if v == t {
			return true
		}
	}
	return false
}

func IsValidPriority(p string) bool {
	for _, v := range SupportedPriorities {
		if v == p {
			return true
		}
	}
	return false
}
