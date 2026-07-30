package deployments

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/modules/deployments/repository"
	"github.com/optikklabs/query/internal/modules/deployments/service"
)

func NewModule(nativeQuerier clickhouse.Conn) *module {
	return &module{
		handler: &Handler{
			service: service.NewService(repository.NewRepository(nativeQuerier)),
		},
	}
}

type module struct {
	handler *Handler
}

func (m *module) Name() string { return "deployments" }

func (m *module) RegisterRoutes(group chi.Router) {
	h := m.handler
	group.Get("/deployments", h.List)
	group.Get("/deployments/{service}/{version}", h.Compare)
	group.Get("/deployments/{service}/{version}/traffic", h.Traffic)
	group.Get("/deployments/{service}/{version}/errors", h.Errors)
	group.Get("/deployments/{service}/{version}/endpoints", h.Endpoints)
	group.Get("/deployments/{service}/{version}/dependencies", h.Dependencies)
}
