package explorer

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

type Config struct {
	Enabled bool
}

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Post("/traces/query", h.Query)
	v1.Post("/traces/enrich", h.EnrichTraces)
	v1.Post("/traces/facets", h.QueryFacets)
	v1.Post("/traces/trend", h.QueryTrend)
	v1.Post("/traces/suggest", h.Suggest)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
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
	RegisterRoutes(group, m.handler)
}
