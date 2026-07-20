package logtrends

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	if h == nil {
		return
	}
	v1.Post("/logs/summary", h.Summary)
	v1.Post("/logs/trend", h.Trend)
}

func NewModule(db clickhouse.Conn) registry.Module {
	m := &module{}
	m.configure(db)
	return m
}

type module struct {
	handler *Handler
}

func (m *module) Name() string { return "logsTrends" }

func (m *module) configure(db clickhouse.Conn) {
	repo := NewRepository(db)
	svc := NewService(repo)
	m.handler = NewHandler(svc)
}

func (m *module) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
