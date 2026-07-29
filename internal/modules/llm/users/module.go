package users

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
)

func NewModule(nativeQuerier clickhouse.Conn) *usersModule {
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
	group.Get("/llm/users/overview", m.handler.Overview)
	group.Post("/llm/users/query", m.handler.Query)
}
