package evaluators

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/app/registry"
)

type Module struct {
	handler *Handler
}

func NewModule(sqlDB *registry.SQLDB, ch clickhouse.Conn) *Module {
	return &Module{handler: NewHandler(NewService(NewRepository(sqlDB, ch)))}
}

func (m *Module) Name() string { return "llm.evaluators" }

func (m *Module) RegisterRoutes(v1 chi.Router) {
	v1.Route("/llm/evaluators", func(r chi.Router) {
		r.Get("/", m.handler.List)
		r.Post("/", m.handler.Create)
		r.Patch("/{id}", m.handler.Update)
		r.Delete("/{id}", m.handler.Delete)
	})
}
