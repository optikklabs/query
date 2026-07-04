// Package onboarding exposes provisioning + first-data status for the CLI wizard.
package onboarding

import (
	"database/sql"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/app/registry"
)

func NewModule(db *sql.DB, ch clickhouse.Conn) registry.Module {
	repo := NewRepository(db, ch)
	return &module{handler: NewHandler(NewService(repo))}
}

type module struct {
	handler *Handler
}

func (m *module) Name() string { return "onboarding" }

func (m *module) RegisterRoutes(group chi.Router) {
	group.Get("/onboarding/status", m.handler.Status)
}
