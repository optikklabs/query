// Package kafka serves the Kafka saturation pages: topic throughput and
// consumer-group partitions from the metrics rollup, and the stream topology
// graph from span_stats.
//
// Layering is enforced by the package structure rather than convention:
//
//	kafka (module, handler) -> service -> repository
//
// with models shared by all three. A handler cannot reach a repository method
// because it does not import that package.
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
