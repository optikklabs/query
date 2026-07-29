package datasets

import (
	"database/sql"
	"github.com/go-chi/chi/v5"

)

type Module struct {
	handler *Handler
}

func NewModule(sqlDB *sql.DB, keys KeyResolver, completer Completer) *Module {
	repo := NewRepository(sqlDB)
	var experiment *ExperimentService
	if keys != nil && completer != nil {
		experiment = NewExperimentService(repo, keys, completer)
	}
	return &Module{handler: NewHandler(NewService(repo), experiment)}
}

func (m *Module) Name() string { return "llm.datasets" }

func (m *Module) RegisterRoutes(v1 chi.Router) {
	v1.Route("/llm/datasets", func(r chi.Router) {
		r.Get("/", m.handler.List)
		r.Post("/", m.handler.Create)
		r.Get("/{id}", m.handler.Get)
		r.Delete("/{id}", m.handler.Delete)
		r.Post("/{id}/items", m.handler.AddItems)
		r.Post("/{id}/runs", m.handler.RunExperiment)
	})
	v1.Get("/llm/runs/{runId}", m.handler.GetRun)
}
