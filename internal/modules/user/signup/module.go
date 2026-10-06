package signup

import (
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/infra/token"
)

func NewModule(service *Service, tokens *token.Service) *Module {
	return &Module{handler: NewHandler(service, tokens)}
}

type Module struct {
	handler *Handler
}

func (m *Module) Name() string { return "user-signup" }

func (m *Module) RegisterRoutes(group chi.Router) {
	group.Post("/auth/signup", m.handler.Signup)
	group.Post("/auth/verify-email", m.handler.VerifyEmail)
}
