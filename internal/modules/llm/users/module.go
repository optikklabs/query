package users

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Get("/llm/users/overview", h.Overview)
	v1.Post("/llm/users/query", h.Query)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	m := &usersModule{}
	m.configure(nativeQuerier)
	return m
}

type usersModule struct {
	handler *Handler
}

func (m *usersModule) Name() string { return "llmUsers" }

func (m *usersModule) configure(db clickhouse.Conn) {
	m.handler = NewHandler(NewService(NewRepository(db)))
}

func (m *usersModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
