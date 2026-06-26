package explorer

import "time"

// ClickHouse scan DTOs match column aliases in repository queries.
// They are separate from API models to allow independent evolution.

// metricNameDTO scans the result of ListMetricNames.
type metricNameDTO struct {
	MetricName  string `ch:"metric_name"`
	MetricType  string `ch:"metric_type"`
	Unit        string `ch:"unit"`
	Description string `ch:"description"`
}

type tagKeyDTO struct {
	TagKey string `ch:"tag_key"`
}

type tagValueDTO struct {
	TagValue string `ch:"tag_value"`
	Count    uint64 `ch:"count"`
}

type tagKeyValueDTO struct {
	TagKey   string `ch:"tag_key"`
	TagValue string `ch:"tag_value"`
	Count    uint64 `ch:"count"`
}

type metricKindDTO struct {
	Temporality string `ch:"temporality"`
	IsMonotonic bool   `ch:"is_monotonic"`
	MetricType  string `ch:"metric_type"`
}

type timeseriesPointDTO struct {
	BucketAt  time.Time `ch:"bucket_at"`
	Sum       float64   `ch:"val_sum"`
	Count     uint64    `ch:"val_count"`
	Min       float64   `ch:"val_min"`
	Max       float64   `ch:"val_max"`
	HistSum   float64   `ch:"hist_sum"`
	HistCount uint64    `ch:"hist_count"`
	Quantiles []float64 `ch:"quantiles"`
}
