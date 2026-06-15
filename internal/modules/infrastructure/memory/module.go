package memory

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

func NewHandler(db clickhouse.Conn) *MemoryHandler {
	return &MemoryHandler{
		Service: NewService(NewRepository(db)),
	}
}

func RegisterRoutes(cfg Config, v1 chi.Router, h *MemoryHandler) {
	if !cfg.Enabled || h == nil {
		return
	}
	v1.Route("/infrastructure/memory", func(r chi.Router) {
		r.Get("/avg", h.GetAvgMemory)
		r.Get("/by-instance", h.GetMemoryByInstance)
	})
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &memoryModule{}
	module.configure(nativeQuerier)
	return module
}

type memoryModule struct {
	handler *MemoryHandler
}

func (m *memoryModule) Name() string { return "memory" }

func (m *memoryModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = NewHandler(nativeQuerier)
}

func (m *memoryModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(DefaultConfig(), group, m.handler)
}
