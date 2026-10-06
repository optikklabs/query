package logs

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/modules/logs/repository"
	"github.com/optikklabs/query/internal/modules/logs/service"
)

func NewModule(nativeQuerier clickhouse.Conn) *Module {
	return &Module{
		handler: &Handler{Service: service.NewService(repository.NewRepository(nativeQuerier))},
	}
}

type Module struct {
	handler *Handler
}

func (m *Module) Name() string { return "logs" }

func (m *Module) RegisterRoutes(group chi.Router) {
	h := m.handler
	group.Post("/logs/query", h.Query)
	group.Post("/logs/suggest", h.Suggest)
	group.Post("/logs/facets", h.Facets)
	group.Post("/logs/summary", h.Summary)
	group.Post("/logs/trend", h.Trend)
	group.Get("/logs/trace/{traceID}", h.GetByTrace)
	group.Get("/logs/{id}", h.GetByID)
}
