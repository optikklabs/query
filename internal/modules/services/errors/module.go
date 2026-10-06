package errors

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/modules/services/errors/repository"
	"github.com/optikklabs/query/internal/modules/services/errors/service"
)

func NewModule(nativeQuerier clickhouse.Conn) *Module {
	return &Module{handler: &ErrorHandler{
		Service: service.NewService(repository.NewRepository(nativeQuerier)),
	}}
}

type Module struct {
	handler *ErrorHandler
}

func (m *Module) Name() string { return "servicesErrors" }

func (m *Module) RegisterRoutes(group chi.Router) {
	h := m.handler
	group.Get("/errors/service-error-rate", h.GetServiceErrorRate)
	group.Post("/errors/groups/query", h.QueryErrorGroups)
	group.Post("/errors/facets", h.QueryErrorFacets)
	group.Post("/errors/overview", h.QueryErrorOverview)
	group.Get("/errors/groups/{groupId}", h.GetErrorGroupDetail)
	group.Get("/errors/groups/{groupId}/traces", h.GetErrorGroupTraces)
	group.Get("/errors/groups/{groupId}/timeseries", h.GetErrorGroupTimeseries)
	group.Get("/errors/groups/{groupId}/latest-occurrence", h.GetErrorGroupLatestOccurrence)
	group.Get("/errors/groups/{groupId}/facets", h.GetErrorGroupFacets)

	group.Get("/spans/error-hotspot", h.GetErrorHotspot)
}
