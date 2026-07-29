package notifications

import (
	"database/sql"
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/modules/alerting/dispatch"
)

type Module struct {
	handler *Handler
}

func NewModule(sqlDB *sql.DB) *Module {
	repo := NewRepository(sqlDB)
	dispatcher := dispatch.NewDefaultDispatcher()
	svc := NewService(repo, dispatcher)
	return &Module{handler: NewHandler(svc)}
}

func (m *Module) Name() string { return "alerting.notifications" }

func (m *Module) RegisterRoutes(v1 chi.Router) {
	v1.Route("/notifications", func(r chi.Router) {
		r.Get("/integrations", m.handler.ListIntegrations)

		r.Route("/channels", func(r chi.Router) {
			r.Get("/", m.handler.ListChannels)
			r.Post("/", m.handler.CreateChannel)
			r.Get("/{id}", m.handler.GetChannel)
			r.Put("/{id}", m.handler.UpdateChannel)
			r.Delete("/{id}", m.handler.DeleteChannel)
			r.Post("/{id}/test", m.handler.TestChannel)
		})

		r.Route("/policies", func(r chi.Router) {
			r.Get("/", m.handler.ListPolicies)
			r.Post("/", m.handler.CreatePolicy)
			r.Put("/{id}", m.handler.UpdatePolicy)
			r.Delete("/{id}", m.handler.DeletePolicy)
		})

		r.Route("/templates", func(r chi.Router) {
			r.Get("/", m.handler.ListTemplates)
			r.Post("/", m.handler.CreateTemplate)
			r.Put("/{id}", m.handler.UpdateTemplate)
			r.Delete("/{id}", m.handler.DeleteTemplate)
		})
	})
}
