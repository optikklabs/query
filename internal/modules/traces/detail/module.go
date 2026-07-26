package detail

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Get("/traces/{traceId}", h.GetTraceSummary)
	v1.Get("/traces/{traceId}/span-events", h.GetSpanEvents)
	v1.Get("/traces/{traceId}/spans/{spanId}/attributes", h.GetSpanAttributes)
	v1.Get("/traces/{traceId}/related", h.GetRelatedTraces)
	v1.Get("/traces/{traceId}/spans", h.GetTraceSpans)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	m := &tracesDetailModule{}
	m.configure(nativeQuerier)
	return m
}

type tracesDetailModule struct {
	handler *Handler
}

func (m *tracesDetailModule) Name() string { return "tracesDetail" }

func (m *tracesDetailModule) configure(db clickhouse.Conn) {
	repo := NewRepository(db)
	svc := NewService(repo)
	m.handler = NewHandler(svc)
}

func (m *tracesDetailModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
