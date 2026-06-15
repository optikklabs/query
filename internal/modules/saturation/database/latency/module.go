package latency

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
	RegisterRoutes(DefaultConfig(), group, m.handler)
}
