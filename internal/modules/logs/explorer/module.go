// Package explorer implements the logs query endpoint for searching logs.
package explorer

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Post("/logs/query", h.Query)
	v1.Post("/logs/suggest", h.Suggest)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	m := &logsExplorerModule{}
	m.configure(nativeQuerier)
	return m
}

type logsExplorerModule struct {
	handler *Handler
}

func (m *logsExplorerModule) Name() string { return "logsExplorer" }

func (m *logsExplorerModule) configure(db clickhouse.Conn) {
	repo := NewRepository(db)
	svc := NewService(repo)
	m.handler = NewHandler(svc)
}

func (m *logsExplorerModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
