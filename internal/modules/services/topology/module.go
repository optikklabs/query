package topology

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
)

func NewModule(nativeQuerier clickhouse.Conn) *Module {
	m := &Module{}
	m.handler = &Handler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
	return m
}

type Module struct {
	handler *Handler
}

func (m *Module) Name() string { return "services_topology" }

func (m *Module) RegisterRoutes(group chi.Router) {
	group.Get("/services/topology", m.handler.GetTopology)
}
