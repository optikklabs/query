package device

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

func (m *Module) Name() string { return "user-device" }

func (m *Module) RegisterRoutes(group chi.Router) {
	group.Post("/auth/device/code", m.handler.DeviceCode)
	group.Post("/auth/device/token", m.handler.DeviceToken)
	group.Post("/auth/device/approve", m.handler.DeviceApprove)
}
