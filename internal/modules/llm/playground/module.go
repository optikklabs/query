package playground

import (
	"github.com/go-chi/chi/v5"
)

// Module wires the interactive playground completion endpoint into the v1
// router. It is only registered when a provider-call path is available.
type Module struct {
	handler *Handler
}

func NewModule(keys KeyResolver, completer Completer, concurrency int) *Module {
	return &Module{handler: NewHandler(NewService(keys, completer, concurrency))}
}

func (m *Module) Name() string { return "llm.playground" }

func (m *Module) RegisterRoutes(v1 chi.Router) {
	v1.Post("/llm/playground/complete", m.handler.Complete)
}
