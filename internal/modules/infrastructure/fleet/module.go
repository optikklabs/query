package fleet

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Get("/infrastructure/fleet/pods", h.GetFleetPods)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &fleetModule{}
	module.configure(nativeQuerier)
	return module
}

type fleetModule struct {
	handler *Handler
}

func (m *fleetModule) Name() string { return "fleet" }

func (m *fleetModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &Handler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *fleetModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
