// Package logfacets provides top-N facet buckets per dimension.
// It is also exposed as a Service method for external callers.
package logfacets //nolint:revive,stylecheck

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Post("/logs/facets", h.Facets)
}

func NewModule(db clickhouse.Conn) registry.Module {
	m := &module{}
	m.configure(db)
	return m
}

type module struct {
	handler *Handler
}

func (m *module) Name() string { return "logsFacets" }

func (m *module) configure(db clickhouse.Conn) {
	repo := NewRepository(db)
	svc := NewService(repo)
	m.handler = NewHandler(svc)
}

func (m *module) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
