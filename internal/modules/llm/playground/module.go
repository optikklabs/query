package playground

import (
	"github.com/go-chi/chi/v5"
)

type Module struct {
	handler *Handler
}

func NewModule(keys KeyResolver, completer Completer) *Module {
	return &Module{handler: NewHandler(NewService(keys, completer))}
}

func (m *Module) Name() string { return "llm.playground" }

func (m *Module) RegisterRoutes(v1 chi.Router) {
	v1.Post("/llm/playground/complete", m.handler.Complete)
}
