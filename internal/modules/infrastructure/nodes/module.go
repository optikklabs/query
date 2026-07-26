package nodes

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *NodeHandler) {
	v1.Get("/infrastructure/nodes", h.GetInfrastructureNodes)
	v1.Get("/infrastructure/nodes/summary", h.GetInfrastructureNodeSummary)
	v1.Get("/infrastructure/nodes/{host}/services", h.GetInfrastructureNodeServices)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &nodesModule{}
	module.configure(nativeQuerier)
	return module
}

type nodesModule struct {
	handler *NodeHandler
}

func (m *nodesModule) Name() string { return "nodes" }

func (m *nodesModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &NodeHandler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *nodesModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
