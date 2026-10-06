package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/optikklabs/query/internal/config"
)

type App struct {
	Config  config.Config
	Infra   *Infra
	Modules []Module
}

func New(cfg config.Config) (*App, error) {
	infraDeps, err := newInfra(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize infrastructure: %w", err)
	}

	modules := configuredModules(infraDeps)

	return &App{
		Config:  cfg,
		Infra:   infraDeps,
		Modules: modules,
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	a.startBackgroundModules()

	var g run.Group
	runAddContextCancelActor(ctx, &g)
	a.addHTTPServerActor(ctx, &g)
	a.addMetricsServerActor(ctx, &g)

	err := g.Run()
	a.stopBackgroundModules()
	if closeErr := a.Infra.Close(); closeErr != nil {
		slog.WarnContext(ctx, "error closing infrastructure", slog.Any("error", closeErr))
	}

	return normalizeRunError(err)
}

func (a *App) addMetricsServerActor(ctx context.Context, g *run.Group) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{DisableCompression: true},
	))
	srv := &http.Server{
		Addr:              ":" + a.Config.Server.MetricsPort,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	g.Add(func() error {
		return srv.ListenAndServe()
	}, func(error) {
		shutdownServer(ctx, srv, "metrics")
	})
}

func (a *App) startBackgroundModules() {
	for _, mod := range a.Modules {
		if r, ok := mod.(BackgroundRunner); ok {
			r.Start()
		}
	}
}

func (a *App) stopBackgroundModules() {
	for _, mod := range a.Modules {
		if r, ok := mod.(BackgroundRunner); ok {
			if stopErr := r.Stop(); stopErr != nil {
				slog.Warn("error stopping module", slog.String("module", mod.Name()), slog.Any("error", stopErr))
			}
		}
	}
}

func runAddContextCancelActor(ctx context.Context, g *run.Group) {
	ctx, cancel := context.WithCancel(ctx)
	g.Add(func() error { <-ctx.Done(); return ctx.Err() },
		func(error) { cancel() })
}

func (a *App) addHTTPServerActor(ctx context.Context, g *run.Group) {
	srv := &http.Server{
		Addr:         ":" + a.Config.Server.Port,
		Handler:      a.Router(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	g.Add(func() error {
		return srv.ListenAndServe()
	}, func(error) {
		shutdownServer(ctx, srv, "http")
	})
}

// shutdownServer drains srv within a fixed budget. It runs after ctx is
// cancelled, so it keeps ctx's values but not its cancellation.
func shutdownServer(ctx context.Context, srv *http.Server, name string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.WarnContext(ctx, "server shutdown incomplete", slog.String("server", name), slog.Any("error", err))
	}
}

func normalizeRunError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
