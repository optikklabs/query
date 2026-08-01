package database

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	databasehandler "github.com/optikklabs/query/internal/modules/saturation/database/handler"
	"github.com/optikklabs/query/internal/modules/saturation/database/repository"
	"github.com/optikklabs/query/internal/modules/saturation/database/service"
)

func NewModule(nativeQuerier clickhouse.Conn) *module {
	return &module{
		handler: databasehandler.New(service.NewService(repository.NewRepository(nativeQuerier))),
	}
}

type module struct {
	handler *databasehandler.Handler
}

func (m *module) Name() string { return "saturationDatabase" }

func (m *module) RegisterRoutes(group chi.Router) {
	h := m.handler
	group.Get("/saturation/datastores/systems", h.GetDatastoreSystems)
	group.Get("/saturation/database/latency/by-system", h.GetLatencyBySystem)
	group.Get("/saturation/database/ops/by-system", h.GetOpsBySystem)
	group.Get("/saturation/database/query-performance/catalogue", h.GetQueryPerformanceCatalogue)
	group.Get("/saturation/database/query-performance/series", h.GetQueryPerformanceSeries)
	group.Post("/database/queries/query", h.QueryPatterns)
	group.Get("/saturation/database/query-detail/summary", h.GetSummary)
	group.Get("/saturation/database/query-detail/timeseries", h.GetTimeseries)
	group.Get("/saturation/database/query-detail/executions", h.GetExecutions)
}
