package repository

import (
	"github.com/ClickHouse/clickhouse-go/v2"
)

type Repository struct {
	db clickhouse.Conn
}

func NewRepository(db clickhouse.Conn) *Repository { return &Repository{db: db} }
