package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/oklog/run"

	"github.com/optikklabs/query/internal/app/registry"
	"github.com/optikklabs/query/internal/config"
)

type App struct {
	Config  config.Config
	Infra   *Infra
	Modules []registry.Module
}

func New(cfg config.Config) (*App, error) {
	infraDeps, err := newInfra(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize infrastructure: %w", err)
	}

	modules := configuredModules(infraDeps.CH, cfg, infraDeps)

	return &App{
		Config:  cfg,
		Infra:   infraDeps,
		Modules: modules,
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	a.startBackgroundModules()

	var g run.Group
	runAddContextCancelActor(&g, ctx)
	a.addHTTPServerActor(&g)

	err := g.Run()
	a.stopBackgroundModules()
	if closeErr := a.Infra.Close(); closeErr != nil {
		slog.WarnContext(ctx, "error closing infrastructure", slog.Any("error", closeErr))
	}

	return normalizeRunError(err)
}

func (a *App) startBackgroundModules() {
	for _, mod := range a.Modules {
		if r, ok := mod.(registry.BackgroundRunner); ok {
			r.Start()
		}
	}
}

func (a *App) stopBackgroundModules() {
	for _, mod := range a.Modules {
		if r, ok := mod.(registry.BackgroundRunner); ok {
			if stopErr := r.Stop(); stopErr != nil {
				slog.Warn("error stopping module", slog.String("module", mod.Name()), slog.Any("error", stopErr))
			}
		}
	}
}

// runAddContextCancelActor shuts down the run group when ctx is cancelled.
func runAddContextCancelActor(g *run.Group, ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	g.Add(func() error { <-ctx.Done(); return ctx.Err() },
		func(error) { cancel() })
}

func (a *App) addHTTPServerActor(g *run.Group) {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", a.Config.Server.Port),
		Handler:      a.Router(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	g.Add(func() error {
		return srv.ListenAndServe()
	}, func(error) {
		shutCtx, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		srv.Shutdown(shutCtx)
	})
}

func normalizeRunError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
