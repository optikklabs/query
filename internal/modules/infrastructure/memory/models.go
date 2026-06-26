package memory

// HTTP response DTOs.

type MetricValue struct {
	Value float64 `json:"value"`
}

type MemoryMetricNameRow struct {
	MetricName string  `ch:"metric_name"`
	Value      float64 `ch:"value"`
}
