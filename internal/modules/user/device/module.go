package device

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

func (m *module) Name() string { return "user-device" }

func (m *module) RegisterRoutes(group chi.Router) {
	// Device flow: code/token are pre-auth; approve needs the browser session.
	group.Post("/auth/device/code", m.handler.DeviceCode)
	group.Post("/auth/device/token", m.handler.DeviceToken)
	group.Post("/auth/device/approve", m.handler.DeviceApprove)
}
