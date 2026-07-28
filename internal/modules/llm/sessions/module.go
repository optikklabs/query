package sessions

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	m := &sessionsModule{}
	m.configure(nativeQuerier)
	return m
}

type sessionsModule struct {
	handler *Handler
}

func (m *sessionsModule) Name() string { return "llmSessions" }

func (m *sessionsModule) configure(db clickhouse.Conn) {
	m.handler = NewHandler(NewService(NewRepository(db)))
}

func (m *sessionsModule) RegisterRoutes(group chi.Router) {
	group.Get("/llm/sessions/overview", m.handler.Overview)
	group.Post("/llm/sessions/query", m.handler.Query)
	group.Get("/llm/sessions/{sessionId}", m.handler.Detail)
}
