package explorer

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
)

func NewModule(nativeQuerier clickhouse.Conn) *Module {
	repo := NewRepository(nativeQuerier)
	svc := NewService(repo)
	return &Module{handler: NewHandler(svc)}
}

type Module struct {
	handler *Handler
}

func (m *Module) Name() string { return "tracesExplorer" }

func (m *Module) RegisterRoutes(group chi.Router) {
	group.Post("/traces/query", m.handler.Query)
	group.Post("/traces/facets", m.handler.QueryFacets)
	group.Post("/traces/trend", m.handler.QueryTrend)
	group.Post("/traces/suggest", m.handler.Suggest)
}
