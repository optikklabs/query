package cloud

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

func RegisterRoutes(cfg Config, v1 chi.Router, h *Handler) {
	if !cfg.Enabled || h == nil {
		return
	}

	v1.Get("/cloud/inventory", h.GetInventory)
	v1.Get("/cloud/categories", h.GetCategories)
	v1.Get("/cloud/health", h.GetHealth)
	v1.Get("/cloud/restarts", h.GetRestarts)
	v1.Get("/cloud/provider/{provider}/platforms", h.GetProviderPlatforms)
	v1.Get("/cloud/provider/{provider}/accounts", h.GetProviderAccounts)
	v1.Get("/cloud/provider/{provider}/resources", h.GetProviderResources)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
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
	RegisterRoutes(DefaultConfig(), group, m.handler)
}
