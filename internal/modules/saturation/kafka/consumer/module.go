package consumer

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
	v1.Get("/saturation/kafka/consume-rate-by-topic", h.GetConsumeRateByTopic)
	v1.Get("/saturation/kafka/consumer-lag-by-group", h.GetConsumerLagByGroup)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &consumerModule{}
	module.configure(nativeQuerier)
	return module
}

type consumerModule struct {
	handler *Handler
}

func (m *consumerModule) Name() string { return "kafkaConsumer" }

func (m *consumerModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &Handler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *consumerModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(DefaultConfig(), group, m.handler)
}
