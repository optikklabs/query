// Package logs serves the logs pages: the search/list explorer with
// suggestions, facet counts, severity trends, single-log deep links, and
// trace correlation.
//
// Layering is enforced by the package structure rather than convention:
//
//	logs (module, handler) -> service -> repository
//
// with models shared by all three. A handler cannot reach a repository method
// because it does not import that package.
package logs

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/app/registry"
	"github.com/optikklabs/query/internal/modules/logs/repository"
	"github.com/optikklabs/query/internal/modules/logs/service"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Post("/logs/query", h.Query)
	v1.Post("/logs/suggest", h.Suggest)
	v1.Post("/logs/facets", h.Facets)
	v1.Post("/logs/summary", h.Summary)
	v1.Post("/logs/trend", h.Trend)
	v1.Get("/logs/trace/{traceID}", h.GetByTrace)
	v1.Get("/logs/{id}", h.GetByID)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	return &module{
		handler: &Handler{Service: service.NewService(repository.NewRepository(nativeQuerier))},
	}
}

type module struct {
	handler *Handler
}

func (m *module) Name() string { return "logs" }

func (m *module) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
