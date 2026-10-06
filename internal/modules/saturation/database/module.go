package database

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	databasehandler "github.com/optikklabs/query/internal/modules/saturation/database/handler"
	"github.com/optikklabs/query/internal/modules/saturation/database/repository"
	"github.com/optikklabs/query/internal/modules/saturation/database/service"
)

func NewModule(nativeQuerier clickhouse.Conn) *Module {
	return &Module{
		handler: databasehandler.New(service.NewService(repository.NewRepository(nativeQuerier))),
	}
}

type Module struct {
	handler *databasehandler.Handler
}

func (m *Module) Name() string { return "saturationDatabase" }

func (m *Module) RegisterRoutes(group chi.Router) {
	h := m.handler
	group.Get("/saturation/datastores/systems", h.GetDatastoreSystems)
	group.Get("/saturation/database/latency/by-system", h.GetLatencyBySystem)
	group.Get("/saturation/database/query-performance/catalogue", h.GetQueryPerformanceCatalogue)
	group.Get("/saturation/database/query-performance/series", h.GetQueryPerformanceSeries)
	group.Post("/database/queries/query", h.QueryPatterns)
	group.Get("/saturation/database/query-detail/summary", h.GetSummary)
	group.Get("/saturation/database/query-detail/timeseries", h.GetTimeseries)
	group.Get("/saturation/database/query-detail/executions", h.GetExecutions)
}
