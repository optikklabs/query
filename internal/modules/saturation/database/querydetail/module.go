package querydetail

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Get("/saturation/database/query-detail/summary", h.GetSummary)
	v1.Get("/saturation/database/query-detail/timeseries", h.GetTimeseries)
	v1.Get("/saturation/database/query-detail/executions", h.GetExecutions)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &dbQueryDetailModule{}
	module.configure(nativeQuerier)
	return module
}

type dbQueryDetailModule struct {
	handler *Handler
}

func (m *dbQueryDetailModule) Name() string { return "dbQueryDetail" }

func (m *dbQueryDetailModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &Handler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *dbQueryDetailModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
