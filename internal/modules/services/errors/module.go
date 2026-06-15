package errors

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

func RegisterRoutes(cfg Config, v1 chi.Router, h *ErrorHandler) {
	if !cfg.Enabled || h == nil {
		return
	}

	v1.Get("/errors/service-error-rate", h.GetServiceErrorRate)
	v1.Get("/errors/error-volume", h.GetErrorVolume)
	v1.Get("/errors/groups", h.GetErrorGroups)
	v1.Get("/errors/groups/{groupId}", h.GetErrorGroupDetail)
	v1.Get("/errors/groups/{groupId}/traces", h.GetErrorGroupTraces)
	v1.Get("/errors/groups/{groupId}/timeseries", h.GetErrorGroupTimeseries)
	v1.Get("/errors/groups/{groupId}/latest-occurrence", h.GetErrorGroupLatestOccurrence)
	v1.Get("/errors/groups/{groupId}/facets", h.GetErrorGroupFacets)

	v1.Get("/spans/error-hotspot", h.GetErrorHotspot)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &errorsModule{}
	module.configure(nativeQuerier)
	return module
}

type errorsModule struct {
	handler *ErrorHandler
}

func (m *errorsModule) Name() string { return "servicesErrors" }

func (m *errorsModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &ErrorHandler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *errorsModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(DefaultConfig(), group, m.handler)
}
