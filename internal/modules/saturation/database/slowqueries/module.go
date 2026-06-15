package slowqueries

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
	v1.Get("/saturation/database/slow-queries/patterns", h.GetSlowQueryPatterns)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &dbSlowModule{}
	module.configure(nativeQuerier)
	return module
}

type dbSlowModule struct {
	handler *Handler
}

func (m *dbSlowModule) Name() string { return "dbSlow" }

func (m *dbSlowModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &Handler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *dbSlowModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(DefaultConfig(), group, m.handler)
}
