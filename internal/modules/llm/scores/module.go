package scores

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Post("/llm/scores", h.Create)
	v1.Get("/llm/scores/names", h.Names)
	v1.Get("/llm/scores/summary", h.Summary)
	v1.Get("/llm/scores/timeseries", h.Timeseries)
	v1.Get("/llm/scores/distribution", h.Distribution)
}

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
	RegisterRoutes(group, m.handler)
}
