package sessions

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Get("/llm/sessions/overview", h.Overview)
	v1.Post("/llm/sessions/query", h.Query)
	v1.Get("/llm/sessions/{sessionId}", h.Detail)
}

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
	RegisterRoutes(group, m.handler)
}
