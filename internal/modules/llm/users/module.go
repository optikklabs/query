package users

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
)

func NewModule(nativeQuerier clickhouse.Conn) *Module {
	return &Module{handler: NewHandler(NewService(NewRepository(nativeQuerier)))}
}

type Module struct {
	handler *Handler
}

func (m *Module) Name() string { return "llmUsers" }

func (m *Module) RegisterRoutes(group chi.Router) {
	group.Get("/llm/users/overview", m.handler.Overview)
	group.Post("/llm/users/query", m.handler.Query)
}
