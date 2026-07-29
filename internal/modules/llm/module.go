package llm

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
)

func NewModule(nativeQuerier clickhouse.Conn) *llmModule {
	m := &llmModule{}
	m.configure(nativeQuerier)
	return m
}

type llmModule struct {
	handler *Handler
}

func (m *llmModule) Name() string { return "llm" }

func (m *llmModule) configure(db clickhouse.Conn) {
	repo := NewRepository(db)
	svc := NewService(repo)
	m.handler = NewHandler(svc)
}

func (m *llmModule) RegisterRoutes(group chi.Router) {
	h := m.handler
	group.Get("/llm/overview", h.Overview)
	group.Get("/llm/apps", h.Apps)
	group.Get("/llm/models", h.Models)
	group.Get("/llm/pricing", h.Pricing)
	group.Get("/llm/timeseries", h.Timeseries)
	group.Get("/llm/cost/breakdown", h.CostBreakdown)
	group.Post("/llm/traces/query", h.TracesQuery)
	group.Get("/llm/traces/{traceId}", h.TraceDetail)
	group.Get("/llm/traces/{traceId}/spans/{spanId}/io", h.SpanIO)
}
