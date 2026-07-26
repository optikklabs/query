package hosts

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Get("/infrastructure/hosts", h.GetHosts)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &hostsModule{}
	module.configure(nativeQuerier)
	return module
}

type hostsModule struct {
	handler *Handler
}

func (m *hostsModule) Name() string { return "infrastructureHosts" }

func (m *hostsModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &Handler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *hostsModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
