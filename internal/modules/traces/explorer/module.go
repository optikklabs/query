package explorer

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
)

func NewModule(nativeQuerier clickhouse.Conn) *tracesExplorerModule {
	m := &tracesExplorerModule{}
	m.configure(nativeQuerier)
	return m
}

type tracesExplorerModule struct {
	handler *Handler
}

func (m *tracesExplorerModule) Name() string { return "tracesExplorer" }

func (m *tracesExplorerModule) configure(db clickhouse.Conn) {
	repo := NewRepository(db)
	svc := NewService(repo)
	m.handler = NewHandler(svc)
}

func (m *tracesExplorerModule) RegisterRoutes(group chi.Router) {
	group.Post("/traces/query", m.handler.Query)
	group.Post("/traces/facets", m.handler.QueryFacets)
	group.Post("/traces/trend", m.handler.QueryTrend)
	group.Post("/traces/suggest", m.handler.Suggest)
}
