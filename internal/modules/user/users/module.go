package users

import (
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/infra/middleware"
)

func NewModule(service *Service) *Module {
	return &Module{handler: NewHandler(service)}
}

type Module struct {
	handler *Handler
}

func (m *Module) Name() string { return "user-users" }

func (m *Module) RegisterRoutes(group chi.Router) {
	group.Group(func(r chi.Router) {
		r.Use(middleware.RequireAdmin)
		r.Post("/users", m.handler.CreateUser)
		r.Get("/users", m.handler.ListUsers)
		r.Patch("/users/{id}/role", m.handler.UpdateUserRole)
		r.Delete("/users/{id}", m.handler.RemoveUser)
	})
}
