// Package repository owns every ClickHouse read in the trace detail domain:
// the trace summary and span list, span events and attributes, related
// traces, the critical and error paths, and the per-trace service map.
//
// Every read here is scoped to one trace id inside one time range, which is
// what makes partition pruning possible — see boundedTraceArgs.
package repository

import (
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

// boundedTraceArgs binds the identity and time range of a trace for partition
// pruning. Nine of the domain's ten reads take exactly these four binds; it
// existed byte-identically in three packages before they were merged.
func boundedTraceArgs(tenantID int64, traceID string, startMs, endMs int64) []any {
	return []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("traceID", traceID),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
	}
}
