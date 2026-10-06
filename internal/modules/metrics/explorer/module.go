package explorer

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
)

func NewModule(nativeQuerier clickhouse.Conn) *Module {
	return &Module{handler: &Handler{
		Service: NewService(NewRepository(nativeQuerier)),
	}}
}

type Module struct {
	handler *Handler
}

func (m *Module) Name() string { return "metricsExplorer" }

func (m *Module) RegisterRoutes(group chi.Router) {
	group.Route("/metrics", func(r chi.Router) {
		r.Get("/names", m.handler.ListMetricNames)
		r.Get("/{metricName}/tags", m.handler.ListTags)
		r.Post("/explorer/query", m.handler.Query)
	})
}
