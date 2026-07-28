package infrastructure

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/app/registry"
	"github.com/optikklabs/query/internal/modules/infrastructure/repository"
	"github.com/optikklabs/query/internal/modules/infrastructure/service"
)

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	return &module{
		handler: &Handler{Service: service.NewService(repository.NewRepository(nativeQuerier))},
	}
}

type module struct {
	handler *Handler
}

func (m *module) Name() string { return "infrastructure" }

func (m *module) RegisterRoutes(group chi.Router) {
	h := m.handler
	group.Route("/infrastructure/cpu", func(r chi.Router) {
		r.Get("/avg", h.GetAvgCPU)
		r.Get("/by-instance", h.GetCPUByInstance)
	})
	group.Route("/infrastructure/memory", func(r chi.Router) {
		r.Get("/avg", h.GetAvgMemory)
		r.Get("/by-instance", h.GetMemoryByInstance)
	})
	group.Get("/infrastructure/hosts", h.GetHosts)
	group.Get("/infrastructure/hosts/{host}/overview", h.GetHostOverview)
	group.Get("/infrastructure/hosts/{host}/series", h.GetHostSeries)
	group.Get("/infrastructure/pods/{pod}/overview", h.GetPodOverview)
	group.Get("/infrastructure/pods/{pod}/series", h.GetPodSeries)
	group.Get("/infrastructure/fleet/pods", h.GetFleetPods)
	group.Get("/infrastructure/nodes", h.GetInfrastructureNodes)
	group.Get("/infrastructure/nodes/summary", h.GetInfrastructureNodeSummary)
	group.Get("/infrastructure/nodes/{host}/services", h.GetInfrastructureNodeServices)
}
