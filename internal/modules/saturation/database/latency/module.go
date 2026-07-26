package latency

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Get("/saturation/database/latency/by-system", h.GetLatencyBySystem)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &dbLatencyModule{}
	module.configure(nativeQuerier)
	return module
}

type dbLatencyModule struct {
	handler *Handler
}

func (m *dbLatencyModule) Name() string { return "dbLatency" }

func (m *dbLatencyModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &Handler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *dbLatencyModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
