package signup

import (
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/infra/token"
)

func NewModule(service *Service, tokens *token.Service) *module {
	return &module{handler: NewHandler(service, tokens)}
}

type module struct {
	handler *Handler
}

func (m *module) Name() string { return "user-signup" }

func (m *module) RegisterRoutes(group chi.Router) {
	group.Post("/auth/signup", m.handler.Signup)
	group.Post("/auth/verify-email", m.handler.VerifyEmail)
}
