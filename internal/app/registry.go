package app

import (
	"database/sql"

	"github.com/go-chi/chi/v5"
)

type SQLDB = sql.DB

type Module interface {
	Name() string
	RegisterRoutes(r chi.Router)
}

type BackgroundRunner interface {
	Start()
	Stop() error
}
