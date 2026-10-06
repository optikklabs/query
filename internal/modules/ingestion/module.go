package ingestion

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/config"
)

type Module struct {
	handler *Handler
}

func NewModule(db clickhouse.Conn, billing config.BillingConfig) *Module {
	return &Module{handler: NewHandler(NewService(NewRepository(db), NewConfig(billing)))}
}

func (m *Module) Name() string { return "ingestion" }

func (m *Module) RegisterRoutes(group chi.Router) {
	group.Get("/ingestion/overview", m.handler.Overview)
}
