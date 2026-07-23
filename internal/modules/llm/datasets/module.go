package datasets

import (
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/app/registry"
)

// Module wires dataset, item, and experiment-run APIs into the v1 router. The
// experiment runner is optional: when keys/completer are absent, POST runs
// answers 503 rather than being unregistered, keeping the contract stable.
type Module struct {
	handler *Handler
}

// NewModule builds the read/CRUD surface. keys and completer may be nil, which
// disables synchronous experiment execution.
func NewModule(sqlDB *registry.SQLDB, keys KeyResolver, completer Completer) *Module {
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
