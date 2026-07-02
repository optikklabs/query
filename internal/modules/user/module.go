package user

import (
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
	"github.com/optikklabs/query/internal/infra/middleware"
	"github.com/optikklabs/query/internal/infra/token"
)

type Config struct {
	Enabled bool
}

func DefaultConfig() Config {
	return Config{Enabled: true}
}

// RegisterRoutes sets up routing for both auth and user profile/settings.
func RegisterRoutes(cfg Config, v1 chi.Router, h *Handler) {
	if !cfg.Enabled || h == nil {
		return
	}

	v1.Route("/auth", func(r chi.Router) {
		r.Post("/login", h.Login)
		r.Post("/refresh", h.Refresh)
		r.Post("/logout", h.Logout)
	})

	// Tenant/user provisioning is restricted to platform super-admins.
	v1.Group(func(r chi.Router) {
		r.Use(middleware.RequireAdmin)
		r.Post("/teams", h.CreateTeam)
		r.Post("/users", h.CreateUser)
	})

	v1.Route("/settings", func(r chi.Router) {
		r.Get("/profile", h.GetProfile)
		r.Put("/profile", h.UpdateProfile)
		r.Put("/preferences", h.UpdatePreferences)
	})
}

func NewModule(
	service *Service,
	tokens *token.Service,
) registry.Module {
	return &userModule{
		handler: NewHandler(service, tokens),
	}
}

type userModule struct {
	handler *Handler
}

func (m *userModule) Name() string { return "user" }

func (m *userModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(DefaultConfig(), group, m.handler)
}
