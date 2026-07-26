// Package infrastructure serves the infrastructure pages: CPU and memory
// averages, the host and node lists, the Kubernetes fleet, and the host and
// pod detail pages.
//
// Layering is enforced by the package structure rather than convention:
//
//	infrastructure (module, handler) -> service -> repository
//
// with models shared by all three. A handler cannot reach a repository method
// because it does not import that package. seriesgroup, seriesdefs and
// infraconsts sit beside these as shared vocabulary, not as layers.
package infrastructure

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/app/registry"
	"github.com/optikklabs/query/internal/modules/infrastructure/repository"
	"github.com/optikklabs/query/internal/modules/infrastructure/service"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Route("/infrastructure/cpu", func(r chi.Router) {
		r.Get("/avg", h.GetAvgCPU)
		r.Get("/by-instance", h.GetCPUByInstance)
	})
	v1.Route("/infrastructure/memory", func(r chi.Router) {
		r.Get("/avg", h.GetAvgMemory)
		r.Get("/by-instance", h.GetMemoryByInstance)
	})
	v1.Get("/infrastructure/hosts", h.GetHosts)
	v1.Get("/infrastructure/hosts/{host}/overview", h.GetHostOverview)
	v1.Get("/infrastructure/hosts/{host}/series", h.GetHostSeries)
	v1.Get("/infrastructure/pods/{pod}/overview", h.GetPodOverview)
	v1.Get("/infrastructure/pods/{pod}/series", h.GetPodSeries)
	v1.Get("/infrastructure/fleet/pods", h.GetFleetPods)
	v1.Get("/infrastructure/nodes", h.GetInfrastructureNodes)
	v1.Get("/infrastructure/nodes/summary", h.GetInfrastructureNodeSummary)
	v1.Get("/infrastructure/nodes/{host}/services", h.GetInfrastructureNodeServices)
}

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
	RegisterRoutes(group, m.handler)
}
