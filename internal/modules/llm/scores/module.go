package scores

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
)

func NewModule(nativeQuerier clickhouse.Conn) *Module {
	return &Module{handler: NewHandler(NewService(NewRepository(nativeQuerier)))}
}

type Module struct {
	handler *Handler
}

func (m *Module) Name() string { return "llmScores" }

func (m *Module) RegisterRoutes(group chi.Router) {
	group.Post("/llm/scores", m.handler.Create)
	group.Get("/llm/scores/summary", m.handler.Summary)
}
