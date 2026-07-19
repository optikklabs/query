package topology

import (
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
	"github.com/optikklabs/query/internal/app/registry"
)

func NewModule(nativeQuerier clickhouse.Conn) registry.Module {
	m := &kafkaTopologyModule{}
	m.handler = &Handler{Service: NewService(NewRepository(nativeQuerier))}
	return m
}

type kafkaTopologyModule struct {
	handler *Handler
}

func (m *kafkaTopologyModule) Name() string { return "saturationKafkaTopology" }

func (m *kafkaTopologyModule) RegisterRoutes(group chi.Router) {
	group.Get("/saturation/kafka/clients", m.handler.GetClients)
	group.Get("/saturation/kafka/topology", m.handler.GetTopology)
}
