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

const queryHashPrewhere = `
	    PREWHERE tenant_id = @tenantID
	         AND db_system != ''
	         AND query_hash = @hash
	         AND timestamp >= @start AND timestamp < @end`

func hashArgs(tenantID, startMs, endMs int64, hash string) []any {
	return append(chargs.RangeArgs(tenantID, startMs, endMs), clickhouse.Named("hash", hash))
}
