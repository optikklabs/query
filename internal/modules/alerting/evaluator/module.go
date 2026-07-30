package evaluator

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/modules/alerting/dispatch"
	"github.com/optikklabs/query/internal/modules/alerting/shared/query"
)

type Module struct {
	svc      *Service
	stop     chan struct{}
	stopped  chan struct{}
	once     sync.Once
	interval time.Duration
}

func NewModule(sqlDB *sql.DB, chConn clickhouse.Conn) *Module {
	repo := NewRepository(sqlDB)
	registry := query.Registry{
		Metric: query.NewMetricBackend(chConn),
		APM:    query.NewAPMBackend(chConn),
		Log:    query.NewLogBackend(chConn),
	}
	dispatcher := dispatch.NewDefaultDispatcher()
	svc := NewService(repo, registry, dispatcher)
	return &Module{
		svc:      svc,
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
		interval: 10 * time.Second,
	}
}

func (m *Module) Name() string { return "alerting.evaluator" }

func (m *Module) RegisterRoutes(chi.Router) {}

func (m *Module) Start() {
	go m.run()
}

func (m *Module) Stop() error {
	m.once.Do(func() { close(m.stop) })
	select {
	case <-m.stopped:
	case <-time.After(15 * time.Second):
		slog.Warn("alerting.evaluator: shutdown timed out")
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
		case now := <-t.C:
			ctx, cancel := context.WithTimeout(context.Background(), m.interval)
			if err := m.svc.Tick(ctx, now.UTC()); err != nil {
				slog.Warn("alerting.evaluator: tick failed", slog.Any("error", err))
			}
			cancel()
		}
	}
}
