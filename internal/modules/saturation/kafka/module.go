package kafka

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/app/registry"
	"github.com/optikklabs/query/internal/modules/saturation/kafka/repository"
	"github.com/optikklabs/query/internal/modules/saturation/kafka/service"
)

func RegisterRoutes(v1 chi.Router, h *Handler) {
	v1.Get("/saturation/kafka/topics/throughput", h.GetTopicThroughput)
	v1.Get("/saturation/kafka/groups/partitions", h.GetGroupPartitions)
	v1.Get("/saturation/kafka/clients", h.GetClients)
	v1.Get("/saturation/kafka/topology", h.GetTopology)
}

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	return &module{
		handler: &Handler{Service: service.NewService(repository.NewRepository(nativeQuerier))},
	}
}

type module struct {
	handler *Handler
}

func (m *module) Name() string { return "saturationKafka" }

func (m *module) RegisterRoutes(group chi.Router) {
	RegisterRoutes(group, m.handler)
}
