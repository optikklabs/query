package producer

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
	v1.Get("/saturation/kafka/produce-rate-by-topic", h.GetProduceRateByTopic)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	module := &producerModule{}
	module.configure(nativeQuerier)
	return module
}

type producerModule struct {
	handler *Handler
}

func (m *producerModule) Name() string { return "kafkaProducer" }

func (m *producerModule) configure(nativeQuerier clickhouse.Conn) {
	m.handler = &Handler{
		Service: NewService(NewRepository(nativeQuerier)),
	}
}

func (m *producerModule) RegisterRoutes(group chi.Router) {
	RegisterRoutes(DefaultConfig(), group, m.handler)
}
