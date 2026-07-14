package auth

import (
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
	"github.com/optikklabs/query/internal/infra/token"
)

func NewModule(service *Service, tokens *token.Service) registry.Module {
	return &module{handler: NewHandler(service, tokens)}
}

type module struct {
	handler *Handler
}

func (m *module) Name() string { return "user-auth" }

func (m *module) RegisterRoutes(group chi.Router) {
	group.Post("/auth/login", m.handler.Login)
	group.Post("/auth/refresh", m.handler.Refresh)
	group.Post("/auth/logout", m.handler.Logout)
	group.Post("/auth/forgot-password", m.handler.ForgotPassword)
	group.Post("/auth/reset-password", m.handler.ResetPassword)
	group.Post("/auth/change-password", m.handler.ChangePassword)
}
