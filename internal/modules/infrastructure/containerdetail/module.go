package containerdetail

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

type Config struct {
	Enabled bool
}

func DefaultConfig() Config {
	return Config{Enabled: true}
}

func RegisterRoutes(cfg Config, v1 chi.Router, h *Handler) {
	if !cfg.Enabled || h == nil {
		return
	}

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
	RegisterRoutes(DefaultConfig(), group, m.handler)
}
