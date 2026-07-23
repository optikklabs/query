package scores

import "time"

// CreateScoreRequest is a human/programmatic score submitted from the UI.
type CreateScoreRequest struct {
	TraceID     string   `json:"traceId"`
	SpanID      string   `json:"spanId"`
	Name        string   `json:"name"`
	DataType    string   `json:"dataType"` // numeric | boolean | categorical
	Value       *float64 `json:"value"`
	StringValue string   `json:"stringValue"`
	Comment     string   `json:"comment"`
}

// ScoreName is one distinct score name with its data type.
type ScoreName struct {
	Name     string `json:"name"`
	DataType string `json:"dataType"`
}

type ScoreNamesResponse struct {
	Names []ScoreName `json:"names"`
}

// ScoreSummary aggregates one score name over the window.
type ScoreSummary struct {
	Name     string  `json:"name"`
	DataType string  `json:"dataType"`
	Count    uint64  `json:"count"`
	Mean     float64 `json:"mean"`
}

type ScoreSummaryResponse struct {
	Summaries []ScoreSummary `json:"summaries"`
}

type Point struct {
	T     int64   `json:"t"`
	Value float64 `json:"value"`
}

type ScoreTimeseriesResponse struct {
	Name   string  `json:"name"`
	Points []Point `json:"points"`
}

// DistributionBucket is one bar of a numeric score histogram.
type DistributionBucket struct {
	Label string `json:"label"`
	Count uint64 `json:"count"`
}

type ScoreDistributionResponse struct {
	Name    string               `json:"name"`
	Buckets []DistributionBucket `json:"buckets"`
}

// --- ClickHouse row structs ---

type nameRow struct {
	Name     string `ch:"name"`
	DataType string `ch:"data_type"`
}

type summaryRow struct {
	Name     string  `ch:"name"`
	DataType string  `ch:"data_type"`
	Count    uint64  `ch:"cnt"`
	Mean     float64 `ch:"mean"`
}

type bucketRow struct {
	BucketAt time.Time `ch:"bucket_at"`
	Mean     float64   `ch:"mean"`
}

type histRow struct {
	Bucket uint8  `ch:"bucket"`
	Count  uint64 `ch:"cnt"`
}
