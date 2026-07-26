// Package repository owns every ClickHouse read in the infrastructure domain:
// CPU and memory utilization, the host and node lists, fleet pods, and the
// host and pod detail pages.
//
// Row types declared here are scan targets, not wire types. They cross into
// service/, which folds them into the API models — they never reach JSON.
package repository

import (
	"github.com/ClickHouse/clickhouse-go/v2"

	"github.com/optikklabs/query/internal/modules/infrastructure/seriesgroup"
)

// Query limits and defaults for node and fleet aggregates.
const (
	MaxNodes    = 200
	MaxServices = 100

	// unknownHost stands in for rows whose host column is empty. Three
	// queries bind it under two different names (@defaultUnknown and
	// @unknownHost); those names are part of the pinned SQL and are left
	// as they were.
	unknownHost = "unknown"

	maxFleetPods = 200
)

// Scope columns on metrics_series for the two detail pages. These are column
// names supplied here, never by request input.
const (
	hostScopeColumn = "host"
	podScopeColumn  = "pod"
)

type Repository struct {
	db     clickhouse.Conn
	series *seriesgroup.Repository
}

func NewRepository(db clickhouse.Conn) *Repository {
	return &Repository{db: db, series: seriesgroup.NewRepository(db)}
}
