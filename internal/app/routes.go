package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/optikklabs/query/internal/infra/middleware"
	"github.com/optikklabs/query/internal/shared/httputil"
)

func (a *App) Router() http.Handler {
	r := chi.NewRouter()

	a.setupGlobalMiddleware(r)
	r.Use(chimw.Compress(5))
	a.setupHealthRoutes(r)
	a.setupAPIRoutes(r)

	return r
}

func (a *App) setupGlobalMiddleware(r chi.Router) {
	r.Use(middleware.RequestID())
	r.Use(middleware.ErrorRecovery())
	// Traefik is the sole public proxy. Read the address it appends to XFF
	// without trusting any client-supplied entries earlier in the chain.
	r.Use(chimw.ClientIPFromXFFTrustedProxies(1))
	r.Use(middleware.HTTPMetricsMiddleware())
	r.Use(middleware.CORSMiddleware(a.Config.Server.AllowedOrigins))
}

func (a *App) setupHealthRoutes(r chi.Router) {
	r.Get("/health", a.healthLive)
	r.Get("/health/live", a.healthLive)
	r.Get("/health/ready", a.healthReady)
}

func (a *App) setupAPIRoutes(r chi.Router) {
	r.Route(httputil.APIV1Base, func(r chi.Router) {
		r.Use(middleware.TenantMiddleware(a.Infra.Tokens))
		r.Use(middleware.TenantRateLimit(100, 200))
		for _, mod := range a.Modules {
			mod.RegisterRoutes(r)
		}
	})
}
