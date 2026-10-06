package infrastructure

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/modules/infrastructure/repository"
	"github.com/optikklabs/query/internal/modules/infrastructure/service"
)

func NewModule(nativeQuerier clickhouse.Conn) *Module {
	return &Module{
		handler: &Handler{Service: service.NewService(repository.NewRepository(nativeQuerier))},
	}
}

type Module struct {
	handler *Handler
}

func (m *Module) Name() string { return "infrastructure" }

func (m *Module) RegisterRoutes(group chi.Router) {
	h := m.handler
	group.Get("/infrastructure/cpu/avg", h.GetAvgCPU)
	group.Get("/infrastructure/memory/avg", h.GetAvgMemory)
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
