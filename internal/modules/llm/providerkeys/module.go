package providerkeys

import (
	"github.com/go-chi/chi/v5"
)

// Module wires encrypted BYO provider-key CRUD into the v1 router. The service
// is injected so the playground module can share it for key resolution.
type Module struct {
	handler *Handler
}

func NewModule(svc *Service) *Module {
	return &Module{handler: NewHandler(svc)}
}

func (m *Module) Name() string { return "llm.providerkeys" }

func (m *Module) RegisterRoutes(v1 chi.Router) {
	v1.Route("/llm/provider-keys", func(r chi.Router) {
		r.Get("/", m.handler.List)
		r.Post("/", m.handler.Create)
		r.Delete("/{id}", m.handler.Delete)
	})
}
