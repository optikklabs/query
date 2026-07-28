package prompts

import (
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/app/registry"
)

type Module struct {
	handler *Handler
}

func NewModule(sqlDB *registry.SQLDB) *Module {
	return &Module{handler: NewHandler(NewService(NewRepository(sqlDB)))}
}

func (m *Module) Name() string { return "llm.prompts" }

func (m *Module) RegisterRoutes(v1 chi.Router) {
	v1.Route("/llm/prompts", func(r chi.Router) {
		r.Get("/", m.handler.List)
		r.Post("/", m.handler.Create)
		r.Get("/{name}", m.handler.Get)
		r.Post("/{name}/versions", m.handler.CreateVersion)
		r.Patch("/{name}/versions/{version}", m.handler.UpdateVersion)
	})
}
