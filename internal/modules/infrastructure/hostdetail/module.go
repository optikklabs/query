package hostdetail

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Get("/infrastructure/hosts/{host}/overview", h.GetOverview)
	v1.Get("/infrastructure/hosts/{host}/series", h.GetSeries)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &hostDetailModule{}
	module.configure(nativeQuerier)
	return module
}

type hostDetailModule struct {
	handler *Handler
}

func (m *hostDetailModule) Name() string { return "hostdetail" }

func (m *hostDetailModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &Handler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *hostDetailModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
