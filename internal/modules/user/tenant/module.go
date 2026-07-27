package tenant

import (
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func NewModule(service *Service) registry.Module {
	return &module{handler: NewHandler(service)}
}

type module struct {
	handler *Handler
}

func (m *module) Name() string { return "user-tenant" }

func (m *module) RegisterRoutes(group chi.Router) {
	group.Post("/settings/api-key/rotate", m.handler.RotateAPIKey)
	group.Post("/settings/api-key/revoke", m.handler.RevokeAPIKey)
	group.Post("/settings/tenant/deactivate", m.handler.DeactivateTenant)
}
