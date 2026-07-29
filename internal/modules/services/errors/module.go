package errors

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
)

func NewModule(nativeQuerier clickhouse.Conn) *errorsModule {
	module := &errorsModule{}
	module.configure(nativeQuerier)
	return module
}

type errorsModule struct {
	handler *ErrorHandler
}

func (m *errorsModule) Name() string { return "servicesErrors" }

func (m *errorsModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &ErrorHandler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *errorsModule) RegisterRoutes(group chi.Router) {
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
