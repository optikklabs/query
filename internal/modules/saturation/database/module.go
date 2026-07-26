// Package database serves the datastore saturation pages: the systems
// explorer, latency and ops timeseries, slow-query patterns, and query detail.
//
// Layering is enforced by the package structure rather than convention:
//
//	database (module, handler) -> service -> repository
//
// with models shared by all three. A handler cannot reach a repository method
// because it does not import that package.
package database

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/app/registry"
	"github.com/optikklabs/query/internal/modules/saturation/database/repository"
	"github.com/optikklabs/query/internal/modules/saturation/database/service"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Get("/saturation/datastores/systems", h.GetDatastoreSystems)
	v1.Get("/saturation/database/latency/by-system", h.GetLatencyBySystem)
	v1.Get("/saturation/database/ops/by-system", h.GetOpsBySystem)
	v1.Get("/saturation/database/slow-queries/patterns", h.GetSlowQueryPatterns)
	v1.Get("/saturation/database/query-detail/summary", h.GetSummary)
	v1.Get("/saturation/database/query-detail/timeseries", h.GetTimeseries)
	v1.Get("/saturation/database/query-detail/executions", h.GetExecutions)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	return &module{
		handler: &Handler{Service: service.NewService(repository.NewRepository(nativeQuerier))},
	}
}

type module struct {
	handler *Handler
}

func (m *module) Name() string { return "saturationDatabase" }

func (m *module) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
