package cpu

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func NewHandler(db clickhouse.Conn) *CPUHandler {
	return &CPUHandler{
		Service: NewService(NewRepository(db)),
	}
}

func RegisterRoutes(v1 chi.Router, h *CPUHandler) {
	v1.Route("/infrastructure/cpu", func(r chi.Router) {
		r.Get("/avg", h.GetAvgCPU)
		r.Get("/by-instance", h.GetCPUByInstance)
	})
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &cpuModule{}
	module.configure(nativeQuerier)
	return module
}

type cpuModule struct {
	handler *CPUHandler
}

func (m *cpuModule) Name() string { return "cpu" }

func (m *cpuModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = NewHandler(nativeQuerier)
}

func (m *cpuModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
