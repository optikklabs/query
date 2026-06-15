package paths

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

type Config struct {
	Enabled bool
}

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Get("/traces/{traceId}/critical-path", h.GetCriticalPath)
	v1.Get("/traces/{traceId}/error-path", h.GetErrorPath)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	m := &tracesPathsModule{}
	m.configure(nativeQuerier)
	return m
}

type tracesPathsModule struct {
	handler *Handler
}

func (m *tracesPathsModule) Name() string { return "tracesPaths" }

func (m *tracesPathsModule) configure(db clickhouse.Conn) {
	repo := NewRepository(db)
	svc := NewService(repo)
	m.handler = NewHandler(svc)
}

func (m *tracesPathsModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
