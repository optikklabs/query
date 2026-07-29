package cloud

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
)

func NewModule(nativeQuerier clickhouse.Conn) *cloudModule {
	module := &cloudModule{}
	module.configure(nativeQuerier)
	return module
}

type cloudModule struct {
	handler *Handler
}

func (m *cloudModule) Name() string { return "cloud" }

func (m *cloudModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &Handler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *cloudModule) RegisterRoutes(group chi.Router) {
	h := m.handler
	group.Get("/cloud/inventory", h.GetInventory)
	group.Get("/cloud/categories", h.GetCategories)
	group.Get("/cloud/health", h.GetHealth)
	group.Get("/cloud/restarts", h.GetRestarts)
	group.Get("/cloud/provider/{provider}/platforms", h.GetProviderPlatforms)
	group.Get("/cloud/provider/{provider}/accounts", h.GetProviderAccounts)
	group.Get("/cloud/provider/{provider}/resources", h.GetProviderResources)
}
