package consumer

import "time"

// HTTP response DTOs.

// TopicRatePoint — consume rate per topic per time bucket.
type TopicRatePoint struct {
	Timestamp  string  `json:"timestamp"`
	Topic      string  `json:"topic"`
	RatePerSec float64 `json:"rate_per_sec"`
}

type LagPoint struct {
	Timestamp     string  `json:"timestamp"`
	ConsumerGroup string  `json:"consumer_group"`
	Topic         string  `json:"topic"`
	Lag           float64 `json:"lag"`
}

type TopicCounterRow struct {
	Timestamp time.Time `ch:"timestamp"`
	Topic     string    `ch:"topic"`
	Value     float64   `ch:"value"`
}

type GroupTopicGaugeRow struct {
	Timestamp     time.Time `ch:"timestamp"`
	ConsumerGroup string    `ch:"consumer_group"`
	Topic         string    `ch:"topic"`
	Value         float64   `ch:"value"`
}
