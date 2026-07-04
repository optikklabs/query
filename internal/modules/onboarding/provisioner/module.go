// Package provisioner runs the BackgroundRunner that materializes per-tenant
// otel-collectors for teams whose provisioning_status is still pending.
package provisioner

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/app/registry"
)

// Module is the BackgroundRunner. It registers no HTTP routes; the ticker
// drives Service.Tick. Outside a cluster the runner is disabled.
type Module struct {
	svc      *Service
	stop     chan struct{}
	stopped  chan struct{}
	once     sync.Once
	interval time.Duration
}

func NewModule(sqlDB *registry.SQLDB) *Module {
	m := &Module{
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
		interval: 15 * time.Second,
	}
	applier, err := NewInClusterApplier()
	if err != nil {
		slog.Info("onboarding.provisioner: disabled (not in cluster)", slog.Any("reason", err))
		return m
	}
	m.svc = NewService(NewRepository(sqlDB), applier)
	return m
}

func (m *Module) Name() string { return "onboarding.provisioner" }

func (m *Module) RegisterRoutes(chi.Router) {}

func (m *Module) Start() {
	if m.svc == nil {
		close(m.stopped)
		return
	}
	go m.run()
}

func (m *Module) Stop() error {
	m.once.Do(func() { close(m.stop) })
	select {
	case <-m.stopped:
	case <-time.After(15 * time.Second):
		slog.Warn("onboarding.provisioner: shutdown timed out")
	}
	return nil
}

func (m *Module) run() {
	defer close(m.stopped)
	t := time.NewTicker(m.interval)
	defer t.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(context.Background(), m.interval)
			if err := m.svc.Tick(ctx); err != nil {
				slog.Warn("onboarding.provisioner: tick failed", slog.Any("error", err))
			}
			cancel()
		}
	}
}
