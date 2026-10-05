package app

import "github.com/go-chi/chi/v5"

type Module interface {
	Name() string
	RegisterRoutes(r chi.Router)
}

type BackgroundRunner interface {
	Start()
	Stop() error
}
