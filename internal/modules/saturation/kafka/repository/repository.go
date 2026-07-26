// Package repository owns every ClickHouse read in the Kafka saturation
// domain: topic throughput and consumer-group partitions from the metrics
// rollup, and the client roster and stream graph from the span_stats rollup.
package repository

import "github.com/ClickHouse/clickhouse-go/v2"

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}
