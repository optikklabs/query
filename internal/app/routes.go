package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/optikklabs/query/internal/infra/middleware"
	"github.com/optikklabs/query/internal/shared/httputil"
)

var readyCache = newHealthCache()

func (a *App) Router() http.Handler {
	r := chi.NewRouter()

	a.setupGlobalMiddleware(r)
	a.setupMetricsRoute(r)

	// Compression applies to everything except /metrics.
	r.Group(func(r chi.Router) {
		r.Use(chimw.Compress(5))
		a.setupHealthRoutes(r)
		a.setupAPIRoutes(r)
	})

	return r
}

func (a *App) setupMetricsRoute(r chi.Router) {
	r.Method(http.MethodGet, "/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{DisableCompression: true}))
}

func (a *App) setupGlobalMiddleware(r chi.Router) {
	r.Use(middleware.RequestID())
	r.Use(middleware.ErrorRecovery())
	r.Use(chimw.RealIP)
	r.Use(middleware.HTTPMetricsMiddleware())
	r.Use(middleware.CORSMiddleware(a.Config.Server.AllowedOrigins))
	r.Use(middleware.BodyLimitMiddleware(10 * 1024 * 1024))
}

func (a *App) setupHealthRoutes(r chi.Router) {
	r.Get("/health", a.healthLive)
	r.Get("/health/live", a.healthLive)
	r.Get("/health/ready", a.healthReady)
}

func (a *App) setupAPIRoutes(r chi.Router) {
	r.Route(httputil.APIV1Base, func(r chi.Router) {
		r.Use(middleware.TenantMiddleware(a.Infra.Tokens))
		r.Use(middleware.ExpensiveQueryLimit(a.Config.ExpensiveQueryConcurrency()))
		for _, mod := range a.Modules {
			mod.RegisterRoutes(r)
		}
	})
}

func (a *App) healthLive(w http.ResponseWriter, _ *http.Request) {
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) healthReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	res := readyCache.get(ctx, a.probeReady)

	if !res.ready {
		payload := map[string]string{"status": "not_ready"}
		if res.mysqlErr != "" {
			payload["mysql"] = res.mysqlErr
		}
		if res.chErr != "" {
			payload["clickhouse"] = res.chErr
		}
		httputil.WriteJSON(w, http.StatusServiceUnavailable, payload)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready", "mysql": "ok", "clickhouse": "ok"})
}

func (a *App) probeReady(ctx context.Context) *healthResult {
	res := &healthResult{}
	if err := a.Infra.DB.Ping(); err != nil {
		res.mysqlErr = err.Error()
		return res
	}
	if err := a.Infra.CH.Ping(ctx); err != nil {
		slog.ErrorContext(ctx, "health check failed", slog.String("service", "clickhouse"), slog.String("error", err.Error()))
		res.chErr = err.Error()
		return res
	}
	res.ready = true
	return res
}
