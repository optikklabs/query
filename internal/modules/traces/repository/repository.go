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

func boundedTraceArgs(tenantID int64, traceID string, startMs, endMs int64) []any {
	return []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("traceID", traceID),
		clickhouse.Named("start", time.UnixMilli(startMs)),
		clickhouse.Named("end", time.UnixMilli(endMs)),
	}
}
