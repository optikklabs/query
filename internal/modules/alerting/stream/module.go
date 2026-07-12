package stream

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/optikklabs/query/internal/app/registry"
	"github.com/optikklabs/query/internal/config"
)

// Module is driven solely by records from ingest's Kafka metrics topic; there
// is intentionally no database polling loop or scheduled evaluation job.
type Module struct {
	repo   *Repository
	cfg    config.AlertingKafkaConfig
	client *kgo.Client
	stop   chan struct{}
	done   chan struct{}
	once   sync.Once
}

func NewModule(db *registry.SQLDB, cfg config.AlertingKafkaConfig) *Module {
	return &Module{repo: NewRepository(db), cfg: cfg, stop: make(chan struct{}), done: make(chan struct{})}
}

func (m *Module) Name() string              { return "alerting.stream" }
func (m *Module) RegisterRoutes(chi.Router) {}

func (m *Module) Start() {
	if !m.cfg.Enabled {
		close(m.done)
		slog.Info("alerting stream evaluator disabled")
		return
	}
	go m.run()
}

func (m *Module) Stop() error {
	m.once.Do(func() { close(m.stop) })
	select {
	case <-m.done:
	case <-time.After(15 * time.Second):
		return fmt.Errorf("alerting stream evaluator shutdown timed out")
	}
	return nil
}

func (m *Module) run() {
	defer close(m.done)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	monitors, states, err := m.repo.LoadActive(ctx)
	cancel()
	if err != nil {
		slog.Error("alerting stream bootstrap failed", slog.Any("error", err))
		return
	}
	engine := NewEngine(monitors, states)
	maxRecords := m.cfg.MaxPollRecords
	if maxRecords <= 0 {
		maxRecords = 1000
	}
	client, err := kgo.NewClient(kgo.SeedBrokers(m.cfg.Brokers()...), kgo.ConsumerGroup(m.cfg.ConsumerGroup), kgo.ConsumeTopics(m.cfg.MetricsTopic()), kgo.DisableAutoCommit(), kgo.Balancers(kgo.CooperativeStickyBalancer()), kgo.FetchMaxWait(2*time.Second))
	if err != nil {
		slog.Error("alerting stream kafka client failed", slog.Any("error", err))
		return
	}
	m.client = client
	defer client.Close()
	slog.Info("alerting stream evaluator started", slog.String("topic", m.cfg.MetricsTopic()), slog.Int("monitors", len(monitors)))
	for {
		select {
		case <-m.stop:
			return
		default:
		}
		fetches := client.PollRecords(context.Background(), maxRecords)
		if fetches.IsClientClosed() {
			return
		}
		if fetches.NumRecords() == 0 {
			continue
		}
		failed := false
		fetches.EachError(func(topic string, partition int32, err error) {
			failed = true
			slog.Warn("alerting stream kafka fetch failed", slog.String("topic", topic), slog.Int("partition", int(partition)), slog.Any("error", err))
		})
		if failed {
			continue
		}
		records := fetches.Records()
		for _, rec := range records {
			event, err := decodeMetricEvent(rec.Value)
			if err != nil {
				slog.Warn("alerting stream dropped malformed metric", slog.String("topic", rec.Topic), slog.Int("partition", int(rec.Partition)), slog.Int64("offset", rec.Offset), slog.Any("error", err))
				continue
			}
			for _, transition := range engine.OnMetric(event) {
				if err := m.repo.PersistTransition(context.Background(), transition); err != nil {
					failed = true
					slog.Error("alerting stream transition persistence failed", slog.Int64("monitor_id", transition.Monitor.ID), slog.Any("error", err))
					break
				}
				engine.Commit(transition)
			}
			if failed {
				break
			}
		}
		if failed {
			continue
		}
		if err := client.CommitRecords(context.Background(), records...); err != nil {
			slog.Warn("alerting stream offset commit failed", slog.Any("error", err))
		}
	}
}
