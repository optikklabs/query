package models

import "time"

type ListRequest struct {
	TenantID int64
	StartMs  int64
	EndMs    int64
}

type DetailRequest struct {
	ListRequest
	Service        string
	Version        string
	Environment    string
	EnvironmentSet bool
	Limit          int
}

type RawDeploymentRow struct {
	Service     string    `ch:"service"`
	Environment string    `ch:"environment"`
	Version     string    `ch:"service_version"`
	FirstSeen   time.Time `ch:"first_seen"`
	Requests    uint64    `ch:"request_total"`
	Errors      uint64    `ch:"error_total"`
	QS          []float64 `ch:"qs"`
}

type Deployment struct {
	Service         string    `json:"service"`
	Environment     string    `json:"environment"`
	Version         string    `json:"version"`
	PreviousVersion *string   `json:"previousVersion"`
	FirstSeen       time.Time `json:"firstSeen"`
	TimelineEnd     time.Time `json:"timelineEnd"`
	TrafficShare    float64   `json:"trafficShare"`
	RequestCount    uint64    `json:"requestCount"`
	ErrorRate       float64   `json:"errorRate"`
	ErrorRateDelta  *float64  `json:"errorRateDelta"`
	P95Ms           float64   `json:"p95Ms"`
	P95DeltaMs      *float64  `json:"p95DeltaMs"`
}

type ListSummary struct {
	DeploymentCount  int        `json:"deploymentCount"`
	ServiceCount     int        `json:"serviceCount"`
	EnvironmentCount int        `json:"environmentCount"`
	LatestFirstSeen  *time.Time `json:"latestFirstSeen"`
}

type ListResponse struct {
	Results      []Deployment `json:"results"`
	Environments []string     `json:"environments"`
	Summary      ListSummary  `json:"summary"`
}

type Window struct {
	CurrentStart  time.Time `json:"currentStart"`
	CurrentEnd    time.Time `json:"currentEnd"`
	BaselineStart time.Time `json:"baselineStart"`
	BaselineEnd   time.Time `json:"baselineEnd"`
}

type Context struct {
	Service         string    `json:"service"`
	Environment     string    `json:"environment"`
	Version         string    `json:"version"`
	BaselineVersion *string   `json:"baselineVersion"`
	FirstSeen       time.Time `json:"firstSeen"`
	Window          Window    `json:"window"`
}

type RawComparisonRow struct {
	CurrentRequests  uint64    `ch:"current_requests"`
	CurrentErrors    uint64    `ch:"current_errors"`
	CurrentQS        []float64 `ch:"current_qs"`
	BaselineRequests uint64    `ch:"baseline_requests"`
	BaselineErrors   uint64    `ch:"baseline_errors"`
	BaselineQS       []float64 `ch:"baseline_qs"`
}

type MetricComparison struct {
	Current      float64  `json:"current"`
	Baseline     *float64 `json:"baseline"`
	Delta        *float64 `json:"delta"`
	DeltaPercent *float64 `json:"deltaPercent"`
}

type ComparisonMetrics struct {
	Requests  MetricComparison `json:"requests"`
	Errors    MetricComparison `json:"errors"`
	ErrorRate MetricComparison `json:"errorRate"`
	P50Ms     MetricComparison `json:"p50Ms"`
	P75Ms     MetricComparison `json:"p75Ms"`
	P90Ms     MetricComparison `json:"p90Ms"`
	P95Ms     MetricComparison `json:"p95Ms"`
	P99Ms     MetricComparison `json:"p99Ms"`
}

type CompareResponse struct {
	Context Context           `json:"context"`
	Metrics ComparisonMetrics `json:"metrics"`
}

type RawTrafficRow struct {
	BucketAt time.Time `ch:"bucket_at"`
	Version  string    `ch:"service_version"`
	Requests uint64    `ch:"request_total"`
}

type TrafficSeries struct {
	Version  string   `json:"version"`
	Requests []uint64 `json:"requests"`
}

type TrafficResponse struct {
	Context    Context         `json:"context"`
	Timestamps []int64         `json:"timestamps"`
	Series     []TrafficSeries `json:"series"`
}

type RawErrorChangeRow struct {
	GroupID       string `ch:"error_group_id"`
	OperationName string `ch:"operation_name"`
	ExceptionType string `ch:"exception_type"`
	CurrentCount  uint64 `ch:"current_count"`
	BaselineCount uint64 `ch:"baseline_count"`
}

type ErrorChange struct {
	GroupID       string `json:"groupId"`
	OperationName string `json:"operationName"`
	ExceptionType string `json:"exceptionType"`
	CurrentCount  uint64 `json:"currentCount"`
	BaselineCount uint64 `json:"baselineCount"`
}

type ErrorChangesResponse struct {
	Context  Context       `json:"context"`
	New      []ErrorChange `json:"new"`
	Resolved []ErrorChange `json:"resolved"`
}

type RawDimensionDiffRow struct {
	Name             string    `ch:"name"`
	CurrentRequests  uint64    `ch:"current_requests"`
	CurrentErrors    uint64    `ch:"current_errors"`
	CurrentQS        []float64 `ch:"current_qs"`
	BaselineRequests uint64    `ch:"baseline_requests"`
	BaselineErrors   uint64    `ch:"baseline_errors"`
	BaselineQS       []float64 `ch:"baseline_qs"`
}

type REDValues struct {
	Requests  uint64  `json:"requests"`
	Errors    uint64  `json:"errors"`
	ErrorRate float64 `json:"errorRate"`
	P95Ms     float64 `json:"p95Ms"`
}

type DimensionDiff struct {
	Name           string     `json:"name"`
	Current        REDValues  `json:"current"`
	Baseline       *REDValues `json:"baseline"`
	RequestDelta   *float64   `json:"requestDelta"`
	ErrorRateDelta *float64   `json:"errorRateDelta"`
	P95DeltaMs     *float64   `json:"p95DeltaMs"`
}

type DimensionDiffResponse struct {
	Context Context         `json:"context"`
	Results []DimensionDiff `json:"results"`
}
