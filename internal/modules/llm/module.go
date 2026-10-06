package llm

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
)

func NewModule(nativeQuerier clickhouse.Conn) *Module {
	repo := NewRepository(nativeQuerier)
	svc := NewService(repo)
	return &Module{handler: NewHandler(svc)}
}

type Module struct {
	handler *Handler
}

func (m *Module) Name() string { return "llm" }

func (m *Module) RegisterRoutes(group chi.Router) {
	h := m.handler
	group.Get("/llm/overview", h.Overview)
	group.Get("/llm/models", h.Models)
	group.Post("/llm/traces/query", h.TracesQuery)
	group.Get("/llm/traces/{traceId}", h.TraceDetail)
	group.Get("/llm/traces/{traceId}/spans/{spanId}/io", h.SpanIO)
}
