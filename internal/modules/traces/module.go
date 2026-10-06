package traces

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/modules/traces/repository"
	"github.com/optikklabs/query/internal/modules/traces/service"
)

func NewModule(nativeQuerier clickhouse.Conn) *Module {
	return &Module{
		handler: &Handler{Service: service.NewService(repository.NewRepository(nativeQuerier))},
	}
}

type Module struct {
	handler *Handler
}

func (m *Module) Name() string { return "traces" }

func (m *Module) RegisterRoutes(group chi.Router) {
	h := m.handler
	group.Get("/traces/{traceId}", h.GetTraceDetail)
	group.Get("/traces/{traceId}/span-events", h.GetSpanEvents)
	group.Get("/traces/{traceId}/spans/{spanId}/attributes", h.GetSpanAttributes)
	group.Get("/traces/{traceId}/related", h.GetRelatedTraces)
}
