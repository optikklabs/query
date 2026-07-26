// Package repository holds every ClickHouse read behind the datastore
// saturation pages, and the raw row types they scan into.
//
// Row types are exported only so the service package can fold them into API
// models. They are not the wire contract and must never be returned from a
// handler: the service converts them to package models first.
package repository

import (
	"github.com/ClickHouse/clickhouse-go/v2"

	"github.com/optikklabs/query/internal/shared/chargs"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db}
}

// queryHashPrewhere restricts a raw-span scan to one tenant's DB spans
// matching a single normalised query hash.
const queryHashPrewhere = `
	    PREWHERE tenant_id = @tenantID
	         AND db_system != ''
	         AND query_hash = @hash
	         AND timestamp BETWEEN @start AND @end`

func hashArgs(tenantID, startMs, endMs int64, hash string) []any {
	return append(chargs.RangeArgs(tenantID, startMs, endMs), clickhouse.Named("hash", hash))
}
