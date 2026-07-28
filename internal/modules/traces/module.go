package traces

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/app/registry"
	"github.com/optikklabs/query/internal/modules/traces/repository"
	"github.com/optikklabs/query/internal/modules/traces/service"
)

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	return &module{
		handler: &Handler{Service: service.NewService(repository.NewRepository(nativeQuerier))},
	}
}

type module struct {
	handler *Handler
}

func (m *module) Name() string { return "traces" }

func (m *module) RegisterRoutes(group chi.Router) {
	h := m.handler
	group.Get("/traces/{traceId}", h.GetTraceDetail)
	group.Get("/traces/{traceId}/span-events", h.GetSpanEvents)
	group.Get("/traces/{traceId}/spans/{spanId}/attributes", h.GetSpanAttributes)
	group.Get("/traces/{traceId}/related", h.GetRelatedTraces)
}
