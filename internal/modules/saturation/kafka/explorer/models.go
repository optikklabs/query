package explorer

// Topic Domains
type TopicThroughputRow struct {
	Topic         string  `ch:"topic"                json:"topic"`
	BytesPerSec   float64 `ch:"bytes_per_sec"        json:"bytesPerSec"`
	BytesTotal    float64 `ch:"bytes_total"           json:"bytesTotal"`
	RecordsPerSec float64 `ch:"records_per_sec"      json:"recordsPerSec"`
	RecordsTotal  float64 `ch:"records_total"         json:"recordsTotal"`
}

type GroupPartitionsRow struct {
	ConsumerGroup      string  `ch:"consumer_group"      json:"consumerGroup"`
	AssignedPartitions float64 `ch:"assigned_partitions" json:"assignedPartitions"`
	TopicCount         uint64  `ch:"topic_count"         json:"topicCount"`
	Members            float64 `ch:"members"             json:"members"`
}
