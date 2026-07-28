package scores

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	m := &scoresModule{}
	m.configure(nativeQuerier)
	return m
}

type scoresModule struct {
	handler *Handler
}

func (m *scoresModule) Name() string { return "llmScores" }

func (m *scoresModule) configure(db clickhouse.Conn) {
	m.handler = NewHandler(NewService(NewRepository(db)))
}

func (m *scoresModule) RegisterRoutes(group chi.Router) {
	group.Post("/llm/scores", m.handler.Create)
	group.Get("/llm/scores/names", m.handler.Names)
	group.Get("/llm/scores/summary", m.handler.Summary)
	group.Get("/llm/scores/timeseries", m.handler.Timeseries)
	group.Get("/llm/scores/distribution", m.handler.Distribution)
}
