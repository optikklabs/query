package topology

// produceEdgeRow is scanned from the per (service, topic) produce aggregation.
type produceEdgeRow struct {
	Service    string    `ch:"service"`
	Topic      string    `ch:"topic"`
	CallCount  uint64    `ch:"call_count"`
	ErrorCount uint64    `ch:"error_count"`
	QS         []float64 `ch:"qs"`
}

type consumeEdgeRow struct {
	Service       string    `ch:"service"`
	Topic         string    `ch:"topic"`
	ConsumerGroup string    `ch:"consumer_group"`
	CallCount     uint64    `ch:"call_count"`
	ErrorCount    uint64    `ch:"error_count"`
	QS            []float64 `ch:"qs"`
}
