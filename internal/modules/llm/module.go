package llm

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Get("/llm/overview", h.Overview)
	v1.Get("/llm/apps", h.Apps)
	v1.Get("/llm/timeseries", h.Timeseries)
	v1.Get("/llm/cost/breakdown", h.CostBreakdown)
	v1.Post("/llm/traces/query", h.TracesQuery)
	v1.Get("/llm/traces/{traceId}", h.TraceDetail)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
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
	RegisterRoutes(group, m.handler)
}
