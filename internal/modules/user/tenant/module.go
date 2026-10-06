package tenant

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

func (m *Module) Name() string { return "user-tenant" }

func (m *Module) RegisterRoutes(group chi.Router) {
	group.Get("/tenants/current/ingestion-endpoints", m.handler.IngestionEndpoints)
	group.Group(func(r chi.Router) {
		r.Use(middleware.RequireAdmin)
		r.Post("/settings/api-key/rotate", m.handler.RotateAPIKey)
		r.Post("/settings/api-key/revoke", m.handler.RevokeAPIKey)
		r.Post("/settings/tenant/deactivate", m.handler.DeactivateTenant)
	})
}
