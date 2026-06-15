// Package logdetail owns GET /api/v1/logs/{id} — single-log deep link by
// base64url(trace_id:span_id:ts_ns).
package logdetail

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	if h == nil {
		return
	}
	v1.Get("/logs/{id}", h.GetByID)
}

func NewModule(db clickhouse.Conn) registry.Module {
	m := &module{}
	m.configure(db)
	return m
}

type module struct {
	handler *Handler
}

func (m *module) Name() string { return "logsDetail" }

func (m *module) configure(db clickhouse.Conn) {
	repo := NewRepository(db)
	svc := NewService(repo)
	m.handler = NewHandler(svc)
}

func (m *module) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
