package tenant

import (
	"github.com/go-chi/chi/v5"
)

func NewModule(service *Service) *module {
	return &module{handler: NewHandler(service)}
}

type module struct {
	handler *Handler
}

func (m *module) Name() string { return "user-tenant" }

func (m *module) RegisterRoutes(group chi.Router) {
	group.Get("/tenants/current/ingestion-endpoints", m.handler.IngestionEndpoints)
	group.Post("/settings/api-key/rotate", m.handler.RotateAPIKey)
	group.Post("/settings/api-key/revoke", m.handler.RevokeAPIKey)
	group.Post("/settings/tenant/deactivate", m.handler.DeactivateTenant)
}
