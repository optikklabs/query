package models

import (
	"time"

	"github.com/optikklabs/query/internal/infra/cursor"
)

type Log struct {
	ID                string             `json:"id"`
	Timestamp         uint64             `json:"timestamp,string"`
	ObservedTimestamp uint64             `json:"observedTimestamp,string"`
	SeverityText      string             `json:"severityText"`
	SeverityNumber    uint8              `json:"severityNumber"`
	SeverityBucket    uint8              `json:"severityBucket"`
	Body              string             `json:"body"`
	TraceID           string             `json:"traceId"`
	SpanID            string             `json:"spanId"`
	TraceFlags        uint32             `json:"traceFlags"`
	ServiceName       string             `json:"serviceName"`
	Host              string             `json:"host"`
	Pod               string             `json:"pod"`
	Container         string             `json:"container"`
	Environment       string             `json:"environment"`
	AttributesString  map[string]string  `json:"attributesString,omitempty"`
	AttributesNumber  map[string]float64 `json:"attributesNumber,omitempty"`
	AttributesBool    map[string]bool    `json:"attributesBool,omitempty"`
	ScopeName         string             `json:"scopeName"`
	ScopeVersion      string             `json:"scopeVersion"`
}

type LogRow struct {
	LogID             string             `ch:"log_id"`
	Timestamp         time.Time          `ch:"timestamp"`
	ObservedTimestamp uint64             `ch:"observed_timestamp"`
	SeverityText      string             `ch:"severity_text"`
	SeverityNumber    uint8              `ch:"severity_number"`
	SeverityBucket    uint8              `ch:"severity_bucket"`
	Body              string             `ch:"body"`
	TraceID           string             `ch:"trace_id"`
	SpanID            string             `ch:"span_id"`
	TraceFlags        uint32             `ch:"trace_flags"`
	ServiceName       string             `ch:"service"`
	Host              string             `ch:"host"`
	Pod               string             `ch:"pod"`
	Container         string             `ch:"container"`
	Environment       string             `ch:"environment"`
	AttributesString  map[string]string  `ch:"attributes_string"`
	AttributesNumber  map[string]float64 `ch:"attributes_number"`
	AttributesBool    map[string]bool    `ch:"attributes_bool"`
	ScopeName         string             `ch:"scope_name"`
	ScopeVersion      string             `ch:"scope_version"`
}

type Cursor struct {
	Timestamp time.Time `json:"ts"`
	LogID     string    `json:"lid"`
}

func (c Cursor) IsZero() bool {
	return c.Timestamp.IsZero() && c.LogID == ""
}

func (c Cursor) Encode() string {
	if c.IsZero() {
		return ""
	}
	return cursor.Encode(c)
}

func DecodeCursor(raw string) (Cursor, bool) {
	return cursor.Decode[Cursor](raw)
}

type FacetValue struct {
	Value string `json:"value"`
	Count uint64 `json:"count"`
}

var SeverityLabels = []string{"UNSET", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

type Facets struct {
	Severity    []string     `json:"severityBucket"`
	Service     []FacetValue `json:"service"`
	Host        []FacetValue `json:"host,omitempty"`
	Pod         []FacetValue `json:"pod,omitempty"`
	Environment []FacetValue `json:"environment,omitempty"`
}

type Summary struct {
	Total  uint64 `json:"total"`
	Errors uint64 `json:"errors"`
	Warns  uint64 `json:"warns"`
}

type TrendBucket struct {
	TimeBucket string `json:"timeBucket"`
	Total      uint64 `json:"total"`
	Error      uint64 `json:"error"`
	Warn       uint64 `json:"warn"`
	Info       uint64 `json:"info"`
	Debug      uint64 `json:"debug"`
}
