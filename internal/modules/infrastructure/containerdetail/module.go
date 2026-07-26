package containerdetail

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Get("/infrastructure/pods/{pod}/overview", h.GetOverview)
	v1.Get("/infrastructure/pods/{pod}/series", h.GetSeries)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &containerDetailModule{}
	module.configure(nativeQuerier)
	return module
}

type containerDetailModule struct {
	handler *Handler
}

func (m *containerDetailModule) Name() string { return "containerdetail" }

func (m *containerDetailModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &Handler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *containerDetailModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
