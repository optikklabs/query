package explorer

import "time"

type metricNameDTO struct {
	MetricName  string `ch:"metric_name"`
	MetricType  string `ch:"metric_type"`
	Unit        string `ch:"unit"`
	Description string `ch:"description"`
	Temporality string `ch:"temporality"`
	IsMonotonic bool   `ch:"is_monotonic"`
	Variants    uint64 `ch:"variants"`
}

type tagValueDTO struct {
	TagValue string `ch:"tag_value"`
	Count    uint64 `ch:"count"`
}

type tagKeyDTO struct {
	TagKey string `ch:"tag_key"`
}

type timeseriesPointDTO struct {
	BucketAt    time.Time `ch:"bucket_at"`
	GroupValues []string  `ch:"group_values"`
	Sum         float64   `ch:"val_sum"`
	Count       uint64    `ch:"val_count"`
	Min         float64   `ch:"val_min"`
	Max         float64   `ch:"val_max"`
	HistSum     float64   `ch:"hist_sum"`
	HistCount   uint64    `ch:"hist_count"`
	Quantiles   []float64 `ch:"quantiles"`
}
