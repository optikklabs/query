package explorer

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Get("/saturation/datastores/systems", h.GetDatastoreSystems)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &saturationExplorerModule{}
	module.configure(nativeQuerier)
	return module
}

type saturationExplorerModule struct {
	handler *Handler
}

func (m *saturationExplorerModule) Name() string { return "saturationDatastoresExplorer" }

func (m *saturationExplorerModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &Handler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *saturationExplorerModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
