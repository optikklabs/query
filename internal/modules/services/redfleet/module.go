package redfleet

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &redFleetModule{}
	module.configure(nativeQuerier)
	return module
}

type redFleetModule struct {
	handler *REDFleetHandler
}

func (m *redFleetModule) Name() string { return "redFleet" }

func (m *redFleetModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &REDFleetHandler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *redFleetModule) RegisterRoutes(group chi.Router) {
	h := m.handler
	group.Get("/spans/red/fleet-overview", h.GetFleetOverview)

	group.Get("/spans/red/request-and-error-rate", h.GetRequestAndErrorRateTimeSeries)
	group.Get("/spans/red/request-rate", h.GetRequestRateTimeSeries)
	group.Get("/spans/red/status-timeseries", h.GetStatusTimeSeries)
	group.Get("/spans/red/latency-percentiles-timeseries", h.GetLatencyPercentilesTimeSeries)
	group.Get("/spans/red/red-by-endpoint", h.GetREDByEndpointTimeSeries)
	group.Get("/spans/red/top-endpoints", h.GetTopEndpointsCombined)
	group.Get("/spans/red/top-db-queries", h.GetTopDBQueriesCombined)

	group.Get("/spans/red/summary", h.GetServiceSummary)
	group.Get("/spans/red/saturation-timeseries", h.GetServiceSaturationTimeSeries)

	group.Get("/spans/red/operation-baseline", h.GetOperationBaseline)
}
