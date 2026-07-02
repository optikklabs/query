package explorer

// Topic Domains
type TopicThroughputRow struct {
	Topic         string  `ch:"topic"                json:"topic"`
	BytesPerSec   float64 `ch:"bytes_per_sec"        json:"bytes_per_sec"`
	BytesTotal    float64 `ch:"bytes_total"           json:"bytes_total"`
	RecordsPerSec float64 `ch:"records_per_sec"      json:"records_per_sec"`
	RecordsTotal  float64 `ch:"records_total"         json:"records_total"`
}

type GroupPartitionsRow struct {
	ConsumerGroup      string  `ch:"consumer_group"      json:"consumer_group"`
	AssignedPartitions float64 `ch:"assigned_partitions" json:"assigned_partitions"`
	TopicCount         uint64  `ch:"topic_count"         json:"topic_count"`
	Members            float64 `ch:"members"             json:"members"`
}
