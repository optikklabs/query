package topology

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	m := &topologyModule{}
	m.handler = &Handler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
	return m
}

type topologyModule struct {
	handler *Handler
}

func (m *topologyModule) Name() string { return "services_topology" }

func (m *topologyModule) RegisterRoutes(group chi.Router) {
	group.Get("/services/topology", m.handler.GetTopology)
}
