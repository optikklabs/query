package models

import (
	"database/sql/driver"
	"encoding/json"
	"slices"

	"github.com/optikklabs/query/internal/shared/sqljson"
)

type MonitorQuery struct {
	Metric *MetricQuery `json:"metric,omitempty"`
	APM    *APMQuery    `json:"apm,omitempty"`
	Log    *LogQuery    `json:"log,omitempty"`
}

func (q *MonitorQuery) Scan(src any) error { return sqljson.Scan(src, q) }

func (q MonitorQuery) Value() (driver.Value, error) { return json.Marshal(q) }

type MetricQuery struct {
	Metric string `json:"metric"`

	Aggregation string `json:"aggregation"`

	WindowSec int `json:"windowSec"`
}

type APMQuery struct {
	Service  string `json:"service"`
	Resource string `json:"resource,omitempty"`

	Track     string `json:"track"`
	WindowSec int    `json:"windowSec"`
}

type LogQuery struct {
	Query string `json:"query"`

	WindowSec int `json:"windowSec"`
}

type NotifyTargets struct {
	ChannelIDs []int64 `json:"channelIds"`
}

func (n *NotifyTargets) Scan(src any) error { return sqljson.Scan(src, n) }

func (n NotifyTargets) Value() (driver.Value, error) { return json.Marshal(n) }

var SupportedMonitorTypes = []string{"metric", "apm", "log"}

var SupportedPriorities = []string{"P1", "P2", "P3", "P4"}

func IsValidType(t string) bool {
	return slices.Contains(SupportedMonitorTypes, t)
}

func IsValidPriority(p string) bool {
	return slices.Contains(SupportedPriorities, p)
}
