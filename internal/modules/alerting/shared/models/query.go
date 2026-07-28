package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

type MonitorQuery struct {
	Metric *MetricQuery `json:"metric,omitempty"`
	APM    *APMQuery    `json:"apm,omitempty"`
	Log    *LogQuery    `json:"log,omitempty"`
}

func (q *MonitorQuery) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, &q)
}

func (q MonitorQuery) Value() (driver.Value, error) {
	return json.Marshal(q)
}

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

	GroupBy   string `json:"groupBy,omitempty"`
	WindowSec int    `json:"windowSec"`
}

type NotifyTargets struct {
	ChannelIDs []int64 `json:"channelIds"`
}

func (n *NotifyTargets) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, &n)
}

func (n NotifyTargets) Value() (driver.Value, error) {
	return json.Marshal(n)
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
