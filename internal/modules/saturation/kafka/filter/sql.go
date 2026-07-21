package filter

// Canonical metrics_series attribute-path expressions for Kafka dimensions.
// These replace the former fixed rollup columns: the dimension now lives in the
// series metadata JSON and is resolved to a fingerprint set before the rollup
// join. Keys match the canonical names written at ingest (see
// ingest/internal/ingestion/metrics/normalize.go).
const (
	AttrTopic         = "attributes['messaging.destination.name']"
	AttrConsumerGroup = "attributes['messaging.consumer.group.name']"
	AttrSystem        = "attributes['messaging.system']"
)
