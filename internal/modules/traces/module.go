// Package traces serves the trace detail page: the trace summary and span
// list, span events and attributes, related traces, the critical and error
// paths, and the per-trace service map.
//
// Layering is enforced by the package structure rather than convention:
//
//	traces (module, handler) -> service -> repository
//
// with models shared by all three. The search/list side of traces keeps its
// own package, traces/explorer, because it is a distinct read surface built
// on traces/filter rather than on a single trace id.
package traces

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/app/registry"
	"github.com/optikklabs/query/internal/modules/traces/repository"
	"github.com/optikklabs/query/internal/modules/traces/service"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Get("/traces/{traceId}", h.GetTraceSummary)
	v1.Get("/traces/{traceId}/span-events", h.GetSpanEvents)
	v1.Get("/traces/{traceId}/spans/{spanId}/attributes", h.GetSpanAttributes)
	v1.Get("/traces/{traceId}/related", h.GetRelatedTraces)
	v1.Get("/traces/{traceId}/spans", h.GetTraceSpans)
	v1.Get("/traces/{traceId}/critical-path", h.GetCriticalPath)
	v1.Get("/traces/{traceId}/error-path", h.GetErrorPath)
	v1.Get("/traces/{traceId}/service-map", h.GetServiceMap)
	v1.Get("/traces/{traceId}/errors", h.GetTraceErrors)
}

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
	RegisterRoutes(group, m.handler)
}
