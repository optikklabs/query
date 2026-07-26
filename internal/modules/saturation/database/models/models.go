// Package models holds the API response types for the datastore saturation
// pages. It is the wire contract: field names and JSON tags here are what the
// frontend consumes, so changes are breaking.
//
// It sits in its own package because both the handler and the service need it,
// and the handler's package imports the service — the two cannot share a
// package without an import cycle.
package models

type DatastoreSystemRow struct {
	System            string  `json:"system"`
	Category          string  `json:"category"`
	QueryCount        int64   `json:"queryCount"`
	AvgLatencyMs      float64 `json:"avgLatencyMs"`
	P95LatencyMs      float64 `json:"p95LatencyMs"`
	ErrorRate         float64 `json:"errorRate"`
	ActiveConnections int64   `json:"activeConnections"`
	ServerHint        string  `json:"serverHint"`
	LastSeen          string  `json:"lastSeen"`
}

type LatencyTimeSeries struct {
	TimeBucket string   `json:"timeBucket"`
	GroupBy    string   `json:"groupBy"`
	P50Ms      *float64 `json:"p50Ms"`
	P95Ms      *float64 `json:"p95Ms"`
	P99Ms      *float64 `json:"p99Ms"`
}

type OpsTimeSeries struct {
	TimeBucket string   `json:"timeBucket"`
	GroupBy    string   `json:"groupBy"`
	OpsPerSec  *float64 `json:"opsPerSec"`
}

type SlowQueryPattern struct {
	QueryHash      string   `json:"queryHash"`
	QueryText      string   `json:"queryText"`
	DBSystem       string   `json:"dbSystem"`
	CollectionName string   `json:"collectionName"`
	Namespace      string   `json:"namespace"`
	Server         string   `json:"server"`
	P50Ms          *float64 `json:"p50Ms"`
	P95Ms          *float64 `json:"p95Ms"`
	P99Ms          *float64 `json:"p99Ms"`
	CallCount      int64    `json:"callCount"`
	ErrorCount     int64    `json:"errorCount"`
}

type ServiceCalls struct {
	Service   string `json:"service"`
	CallCount int64  `json:"callCount"`
}

type QuerySummary struct {
	QueryHash      string         `json:"queryHash"`
	QueryText      string         `json:"queryText"`
	DbSystem       string         `json:"dbSystem"`
	CollectionName string         `json:"collectionName"`
	OperationName  string         `json:"operationName"`
	CallCount      int64          `json:"callCount"`
	ErrorCount     int64          `json:"errorCount"`
	P50Ms          *float64       `json:"p50Ms"`
	P95Ms          *float64       `json:"p95Ms"`
	P99Ms          *float64       `json:"p99Ms"`
	AvgMs          float64        `json:"avgMs"`
	TotalTimeMs    float64        `json:"totalTimeMs"`
	AvgRows        *float64       `json:"avgRows"`
	Services       []ServiceCalls `json:"services"`
}

type QueryTimeseriesPoint struct {
	TimeBucket string   `json:"timeBucket"`
	CallCount  int64    `json:"callCount"`
	ErrorCount int64    `json:"errorCount"`
	AvgMs      *float64 `json:"avgMs"`
	P99Ms      *float64 `json:"p99Ms"`
}

type QueryExecution struct {
	Timestamp  string   `json:"timestamp"`
	TraceID    string   `json:"traceId"`
	SpanID     string   `json:"spanId"`
	DurationMs float64  `json:"durationMs"`
	IsError    bool     `json:"isError"`
	Service    string   `json:"service"`
	Host       string   `json:"host"`
	Rows       *float64 `json:"rows"`
}
