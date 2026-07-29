package ingestion

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/config"
)

func NewModule(db clickhouse.Conn, billing config.BillingConfig) *module {
	m := &module{cfg: NewConfig(billing)}
	m.configure(db)
	return m
}

type module struct {
	cfg     Config
	handler *Handler
}

func (m *module) Name() string { return "ingestion" }

func (m *module) configure(db clickhouse.Conn) {
	repo := NewRepository(db)
	svc := NewService(repo, m.cfg)
	m.handler = NewHandler(svc)
}

func (m *module) RegisterRoutes(group chi.Router) {
	group.Get("/ingestion/overview", m.handler.Overview)
}
