package billing

import (
	"database/sql"
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

)

type Module struct {
	repo     *Repository
	stop     chan struct{}
	stopped  chan struct{}
	once     sync.Once
	interval time.Duration
}

func NewModule(sqlDB *sql.DB) *Module {
	return &Module{
		repo:     NewRepository(sqlDB),
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
		interval: time.Hour,
	}
}

func (m *Module) Name() string { return "billing.trial-sweeper" }

func (m *Module) RegisterRoutes(chi.Router) {}

func (m *Module) Start() {
	go m.run()
}

func (m *Module) Stop() error {
	m.once.Do(func() { close(m.stop) })
	select {
	case <-m.stopped:
	case <-time.After(15 * time.Second):
		slog.Warn("billing.trial-sweeper: shutdown timed out")
	}
	return nil
}

func (m *Module) run() {
	defer close(m.stopped)
	m.sweep()
	t := time.NewTicker(m.interval)
	defer t.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-t.C:
			m.sweep()
		}
	}
}

func (m *Module) sweep() {
	ctx, cancel := context.WithTimeout(context.Background(), m.interval)
	defer cancel()
	suspended, err := m.repo.SuspendExpiredTrials(ctx, time.Now().UTC())
	if err != nil {
		slog.Warn("billing.trial-sweeper: sweep failed", slog.Any("error", err))
		return
	}
	if suspended > 0 {
		slog.Info("billing.trial-sweeper: suspended expired trials",
			slog.Int64("count", suspended))
	}
}
