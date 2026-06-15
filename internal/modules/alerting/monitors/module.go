package monitors

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/app/registry"
	"github.com/optikklabs/query/internal/modules/alerting/shared/query"
)

// Module wires the monitors endpoints into the v1 router.
// It supports CRUD, state actions, and per-monitor query views.
type Module struct {
	handler *Handler
}

func NewModule(sqlDB *registry.SQLDB, chConn clickhouse.Conn) *Module {
	repo := NewRepository(sqlDB)
	svc := NewService(repo)
	queries := query.Registry{
		Metric: query.NewMetricBackend(chConn),
		APM:    query.NewAPMBackend(chConn),
		Log:    query.NewLogBackend(chConn),
	}
	return &Module{handler: NewHandler(svc, queries)}
}

func (m *Module) Name() string { return "alerting.monitors" }

func (m *Module) RegisterRoutes(v1 chi.Router) {
	v1.Route("/monitors", func(r chi.Router) {
		r.Get("/", m.handler.List)
		r.Post("/", m.handler.Create)
		r.Get("/activity", m.handler.Activity)
		r.Get("/{id}", m.handler.Get)
		r.Put("/{id}", m.handler.Update)
		r.Delete("/{id}", m.handler.Delete)
		r.Post("/{id}/ack", m.handler.Ack)
		r.Post("/{id}/mute", m.handler.Mute)
		r.Post("/{id}/unmute", m.handler.Unmute)
		r.Post("/{id}/test", m.handler.Test)
		r.Get("/{id}/events", m.handler.Events)
		r.Get("/{id}/series", m.handler.Series)
		r.Get("/{id}/status-timeline", m.handler.StatusTimeline)
	})
}
