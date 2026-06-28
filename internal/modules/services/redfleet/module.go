package redfleet

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

type Config struct {
	Enabled bool
}

func DefaultConfig() Config {
	return Config{Enabled: true}
}

// RegisterRoutes mounts the fleet/overview RED endpoints. Paths are registered
// flat (not via chi.Route) so all RED endpoints share /spans/red without a
// duplicate-mount panic.
func RegisterRoutes(cfg Config, v1 chi.Router, h *REDFleetHandler) {
	if !cfg.Enabled || h == nil {
		return
	}
	v1.Get("/spans/red/fleet-totals", h.GetFleetTotals)
	v1.Get("/spans/red/services", h.GetFleetServices)
	v1.Get("/spans/red/fleet-overview", h.GetFleetOverview)

	v1.Get("/spans/red/request-and-error-rate", h.GetRequestAndErrorRateTimeSeries)
	v1.Get("/spans/red/request-rate", h.GetRequestRateTimeSeries)
	v1.Get("/spans/red/status-timeseries", h.GetStatusTimeSeries)
	v1.Get("/spans/red/latency-percentiles-timeseries", h.GetLatencyPercentilesTimeSeries)
	v1.Get("/spans/red/red-by-endpoint", h.GetREDByEndpointTimeSeries)
	v1.Get("/spans/red/top-endpoints", h.GetTopEndpointsCombined)
	v1.Get("/spans/red/top-db-queries", h.GetTopDBQueriesCombined)

	// Flat query-param routes for service summary and saturation.
	v1.Get("/spans/red/summary", h.GetServiceSummary)
	v1.Get("/spans/red/saturation-timeseries", h.GetServiceSaturationTimeSeries)

	// Legacy path-param aliases (backwards compat).
	v1.Get("/spans/red/services/{serviceName}/summary", h.GetServiceSummary)
	v1.Get("/spans/red/services/{serviceName}/saturation-timeseries", h.GetServiceSaturationTimeSeries)

	v1.Get("/spans/red/operation-baseline", h.GetOperationBaseline)
}

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
	RegisterRoutes(DefaultConfig(), group, m.handler)
}
