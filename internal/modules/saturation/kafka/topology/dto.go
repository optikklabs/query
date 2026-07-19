package topology

// edgeRow is one (service, topic, consumer group) aggregation. An empty
// ConsumerGroup marks a produce row; anything else is a consume row.
type edgeRow struct {
	Service       string    `ch:"service"`
	Topic         string    `ch:"topic"`
	ConsumerGroup string    `ch:"consumer_group"`
	CallCount     uint64    `ch:"call_count"`
	ErrorCount    uint64    `ch:"error_count"`
	QS            []float64 `ch:"qs"`
}
