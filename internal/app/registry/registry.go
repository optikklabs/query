package registry

import (
	"database/sql"

	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/config"
)

type SQLDB = sql.DB
type AppConfig = config.Config

type Module interface {
	Name() string
	RegisterRoutes(r chi.Router)
}

type BackgroundRunner interface {
	Start()
	Stop() error
}
