package redmetrics

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

func RegisterRoutes(cfg Config, v1 chi.Router, h *REDMetricsHandler) {
	if !cfg.Enabled || h == nil {
		return
	}
	v1.Route("/spans/red", func(r chi.Router) {
		r.Get("/summary", h.GetSummary)
		r.Get("/apdex", h.GetApdex)
		r.Get("/request-and-error-rate", h.GetRequestAndErrorRateTimeSeries)
		r.Get("/status-timeseries", h.GetStatusTimeSeries)
		r.Get("/latency-percentiles-timeseries", h.GetLatencyPercentilesTimeSeries)
		r.Get("/top-endpoints", h.GetTopEndpointsCombined)
		r.Get("/services/{serviceName}/summary", h.GetServiceSummary)
		r.Get("/operation-baseline", h.GetOperationBaseline)
	})
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &redMetricsModule{}
	module.configure(nativeQuerier)
	return module
}

type redMetricsModule struct {
	handler *REDMetricsHandler
}

func (m *redMetricsModule) Name() string { return "redMetrics" }

func (m *redMetricsModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &REDMetricsHandler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *redMetricsModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(DefaultConfig(), group, m.handler)
}
