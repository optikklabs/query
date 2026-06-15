package volume

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
	v1.Get("/saturation/database/ops/by-system", h.GetOpsBySystem)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &dbVolumeModule{}
	module.configure(nativeQuerier)
	return module
}

type dbVolumeModule struct {
	handler *Handler
}

func (m *dbVolumeModule) Name() string { return "dbVolume" }

func (m *dbVolumeModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &Handler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *dbVolumeModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(DefaultConfig(), group, m.handler)
}
