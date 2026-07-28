package repository

import (
	"github.com/ClickHouse/clickhouse-go/v2"

	"github.com/optikklabs/query/internal/modules/infrastructure/seriesgroup"
)

const (
	MaxNodes    = 200
	MaxServices = 100

	unknownHost = "unknown"

	maxFleetPods = 200
)

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
