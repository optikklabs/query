package kafka

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"

	"github.com/optikklabs/query/internal/modules/saturation/kafka/repository"
	"github.com/optikklabs/query/internal/modules/saturation/kafka/service"
)

func NewModule(nativeQuerier clickhouse.Conn) *module {
	return &module{
		handler: &Handler{Service: service.NewService(repository.NewRepository(nativeQuerier))},
	}
}

type module struct {
	handler *Handler
}

func (m *module) Name() string { return "saturationKafka" }

func (m *module) RegisterRoutes(group chi.Router) {
	group.Get("/saturation/kafka/topics/throughput", m.handler.GetTopicThroughput)
	group.Get("/saturation/kafka/groups/partitions", m.handler.GetGroupPartitions)
	group.Get("/saturation/kafka/clients", m.handler.GetClients)
	group.Get("/saturation/kafka/topology", m.handler.GetTopology)
}
