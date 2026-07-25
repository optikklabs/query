package filter

// Kafka client metrics carry topic and consumer group as datapoint attributes,
// so both are resolved from metrics_series to a fingerprint set before the
// rollup join. Keys match the canonical names written at ingest (see
// ingest/internal/ingestion/metrics/normalize.go).
const (
	AttrTopic         = "attributes['messaging.destination.name']"
	AttrConsumerGroup = "attributes['messaging.consumer.group.name']"
)
