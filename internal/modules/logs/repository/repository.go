// Package repository owns every ClickHouse read in the logs domain: the list
// query, facets, trends, single-log lookup, and trace correlation.
//
// Row types declared here are scan targets, not wire types. They cross into
// service/, which folds them into the API models — they never reach JSON.
package repository

import (
	"github.com/ClickHouse/clickhouse-go/v2"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository { return &Repository{db: db} }
