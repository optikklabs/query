package ingestion

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(cfg Config, v1 chi.Router, h *Handler) {
	if !cfg.Enabled || h == nil {
		return
	}
	v1.Get("/ingestion/summary", h.Summary)
	v1.Get("/ingestion/cost", h.Cost)
	v1.Get("/ingestion/timeseries", h.Timeseries)
	v1.Get("/ingestion/services", h.Services)
}

func NewModule(db clickhouse.Conn) registry.Module {
	m := &module{cfg: DefaultConfig()}
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
	RegisterRoutes(m.cfg, group, m.handler)
}
