package volume

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
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
	RegisterRoutes(group, m.handler)
}
