package explorer

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

type Config struct {
	Enabled bool
}

func DefaultConfig() Config {
	return Config{Enabled: true}
}

func RegisterRoutes(cfg Config, v1 chi.Router, h *Handler) {
	if !cfg.Enabled || h == nil {
		return
	}

	v1.Get("/saturation/kafka/topics/throughput", h.GetTopicThroughput)
	v1.Get("/saturation/kafka/topics/lag", h.GetTopicLag)
	v1.Get("/saturation/kafka/topics/consumers", h.GetTopicConsumers)

	v1.Get("/saturation/kafka/groups/partitions", h.GetGroupPartitions)
	v1.Get("/saturation/kafka/groups/commits", h.GetGroupCommits)
	v1.Get("/saturation/kafka/groups/fetches", h.GetGroupFetches)
	v1.Get("/saturation/kafka/groups/health", h.GetGroupHealth)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &kafkaExplorerModule{}
	module.configure(nativeQuerier)
	return module
}

type kafkaExplorerModule struct {
	handler *Handler
}

func (m *kafkaExplorerModule) Name() string { return "saturationKafkaExplorer" }

func (m *kafkaExplorerModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &Handler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *kafkaExplorerModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(DefaultConfig(), group, m.handler)
}
