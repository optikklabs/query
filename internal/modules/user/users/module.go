package users

import (
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
	"github.com/optikklabs/query/internal/infra/middleware"
)

func NewModule(service *Service) registry.Module {
	return &module{handler: NewHandler(service)}
}

type module struct {
	handler *Handler
}

func (m *module) Name() string { return "user-users" }

func (m *module) RegisterRoutes(group chi.Router) {

	group.Group(func(r chi.Router) {
		r.Use(middleware.RequireAdmin)
		r.Post("/users", m.handler.CreateUser)
		r.Get("/users", m.handler.ListUsers)
		r.Patch("/users/{id}/role", m.handler.UpdateUserRole)
		r.Delete("/users/{id}", m.handler.RemoveUser)
	})
}
