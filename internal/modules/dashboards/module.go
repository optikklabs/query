package dashboards

import (
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/app/registry"
)

type Module struct {
	handler *Handler
}

func NewModule(sqlDB *registry.SQLDB) *Module {
	repo := NewRepository(sqlDB)
	svc := NewService(repo)
	return &Module{handler: NewHandler(svc)}
}

func (m *Module) Name() string { return "dashboards" }

func (m *Module) RegisterRoutes(v1 chi.Router) {
	v1.Route("/dashboard-pages", func(r chi.Router) {
		r.Get("/", m.handler.ListPages)
		r.Post("/", m.handler.CreatePage)
		r.Get("/{id}", m.handler.GetPage)
		r.Put("/{id}", m.handler.UpdatePage)
		r.Delete("/{id}", m.handler.DeletePage)
		r.Get("/{id}/dashboards", m.handler.ListWidgets)
		r.Post("/{id}/dashboards", m.handler.CreateWidget)
		r.Put("/{id}/dashboards/{widgetId}", m.handler.UpdateWidget)
		r.Delete("/{id}/dashboards/{widgetId}", m.handler.DeleteWidget)
	})
}
