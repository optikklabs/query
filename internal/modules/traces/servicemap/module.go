package servicemap

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Get("/traces/{traceId}/service-map", h.GetServiceMap)
	v1.Get("/traces/{traceId}/errors", h.GetTraceErrors)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	m := &tracesServicemapModule{}
	m.configure(nativeQuerier)
	return m
}

type tracesServicemapModule struct {
	handler *Handler
}

func (m *tracesServicemapModule) Name() string { return "tracesServicemap" }

func (m *tracesServicemapModule) configure(db clickhouse.Conn) {
	repo := NewRepository(db)
	svc := NewService(repo)
	m.handler = NewHandler(svc)
}

func (m *tracesServicemapModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
