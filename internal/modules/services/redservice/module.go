package redservice

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

// RegisterRoutes mounts the per-service / per-operation RED endpoints. Paths are
// registered flat (not via chi.Route) so redservice and redfleet can share
// /spans/red without a duplicate-mount panic.
func RegisterRoutes(cfg Config, v1 chi.Router, h *REDServiceHandler) {
	if !cfg.Enabled || h == nil {
		return
	}
	v1.Get("/spans/red/services/{serviceName}/summary", h.GetServiceSummary)
	v1.Get("/spans/red/services/{serviceName}/saturation-timeseries", h.GetServiceSaturationTimeSeries)
	v1.Get("/spans/red/operation-baseline", h.GetOperationBaseline)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &redServiceModule{}
	module.configure(nativeQuerier)
	return module
}

type redServiceModule struct {
	handler *REDServiceHandler
}

func (m *redServiceModule) Name() string { return "redService" }

func (m *redServiceModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &REDServiceHandler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *redServiceModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(DefaultConfig(), group, m.handler)
}
