package explorer

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &metricsExplorerModule{}
	module.configure(nativeQuerier)
	return module
}

type metricsExplorerModule struct {
	handler *Handler
}

func (m *metricsExplorerModule) Name() string { return "metricsExplorer" }

func (m *metricsExplorerModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &Handler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *metricsExplorerModule) RegisterRoutes(group chi.Router) {
	group.Route("/metrics", func(r chi.Router) {
		r.Get("/names", m.handler.ListMetricNames)
		r.Get("/{metricName}/tags", m.handler.ListTags)
		r.Post("/explorer/query", m.handler.Query)
	})
}
