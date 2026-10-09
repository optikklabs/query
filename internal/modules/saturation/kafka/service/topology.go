package service

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/optikklabs/query/internal/modules/saturation/kafka/models"
	"github.com/optikklabs/query/internal/modules/saturation/kafka/repository"
	"github.com/optikklabs/query/internal/shared/metrics"
)

func (s *Service) GetTopicThroughput(ctx context.Context, tenantID, startMs, endMs int64, topic string) ([]models.TopicThroughputRow, error) {
	return s.repo.QueryTopicThroughput(ctx, tenantID, startMs, endMs, topic)
}

func (s *Service) GetGroupPartitions(ctx context.Context, tenantID, startMs, endMs int64, group string) ([]models.GroupPartitionsRow, error) {
	return s.repo.QueryGroupPartitions(ctx, tenantID, startMs, endMs, group)
}

func (s *Service) GetClients(ctx context.Context, tenantID, startMs, endMs int64) ([]string, error) {
	clients, err := s.repo.QueryClients(ctx, tenantID, startMs, endMs)
	if err != nil {
		return nil, err
	}
	if clients == nil {
		clients = []string{}
	}
	return clients, nil
}

func (s *Service) GetTopology(ctx context.Context, tenantID, startMs, endMs int64, services []string) (models.TopologyResponse, error) {
	if len(services) == 0 {
		return buildGraph(nil, 1), nil
	}
	rows, err := s.repo.QueryEdges(ctx, tenantID, startMs, endMs, services)
	if err != nil {
		return models.TopologyResponse{}, err
	}

	return buildGraph(rows, float64(endMs-startMs)/1000), nil
}

const (
	kindProducer = "PRODUCER"
	kindConsumer = "CONSUMER"
)

type percentileValues struct {
	p50 float64
	p95 float64
	p99 float64
}

func percentiles(qs []float64) percentileValues {
	var values percentileValues
	if len(qs) > 0 {
		values.p50 = qs[0]
	}
	if len(qs) > 1 {
		values.p95 = qs[1]
	}
	if len(qs) > 2 {
		values.p99 = qs[2]
	}
	return values
}

type nodeAgg struct {
	calls   uint64
	errors  uint64
	latency percentileValues
}

type topicLeader struct {
	service string
	calls   uint64
}

type graphData struct {
	producers      map[string]*nodeAgg
	consumers      map[string]*nodeAgg
	consumerMeta   map[string][2]string
	topicProduce   map[string]uint64
	topicProducers map[string]map[string]struct{}
	topicGroups    map[string]map[string]struct{}
	topicLeaders   map[string]topicLeader
	consumeEdges   map[[2]string]uint64
	edges          []models.StreamEdge
	pathways       []models.Pathway
}

func newGraphData(size int) *graphData {
	return &graphData{
		producers: map[string]*nodeAgg{}, consumers: map[string]*nodeAgg{},
		consumerMeta: map[string][2]string{}, topicProduce: map[string]uint64{},
		topicProducers: map[string]map[string]struct{}{}, topicGroups: map[string]map[string]struct{}{},
		topicLeaders: map[string]topicLeader{}, consumeEdges: map[[2]string]uint64{},
		edges: make([]models.StreamEdge, 0, size), pathways: make([]models.Pathway, 0, size),
	}
}

func (g *graphData) addProducer(row repository.EdgeRow, winSecs float64) {
	g.producers[row.Service] = &nodeAgg{}
	g.topicProduce[row.Topic] += row.CallCount
	addSet(g.topicProducers, row.Topic, row.Service)
	leader, exists := g.topicLeaders[row.Topic]
	if !exists || row.CallCount > leader.calls || (row.CallCount == leader.calls && row.Service < leader.service) {
		g.topicLeaders[row.Topic] = topicLeader{service: row.Service, calls: row.CallCount}
	}
	g.edges = append(g.edges, models.StreamEdge{
		Source: row.Service, Target: row.Topic, Kind: "produce",
		RatePerSec: float64(row.CallCount) / winSecs,
	})
}

func (g *graphData) addConsumer(row repository.EdgeRow, winSecs float64) {
	key := consumerKey(row)
	g.consumers[key] = &nodeAgg{}
	g.consumerMeta[key] = [2]string{row.Service, row.ConsumerGroup}
	if row.ConsumerGroup != "" {
		addSet(g.topicGroups, row.Topic, row.ConsumerGroup)
	}
	g.consumeEdges[[2]string{row.Topic, row.Service}] += row.CallCount
	g.pathways = append(g.pathways, models.Pathway{
		Producer: g.topicLeaders[row.Topic].service, Topic: row.Topic,
		Group: row.ConsumerGroup, Consumer: row.Service,
		ProduceRatePerSec: float64(g.topicProduce[row.Topic]) / winSecs,
		ConsumeRatePerSec: float64(row.CallCount) / winSecs,
		ErrorRate:         metrics.Percentage(row.ErrorCount, row.CallCount),
	})
}

// setNode fills a node the edges registered with its stats, which SQL merged
// over all of the node's topics.
func (g *graphData) setNode(row repository.EdgeRow) {
	nodes, key := g.producers, row.Service
	if row.Kind == kindConsumer {
		nodes, key = g.consumers, consumerKey(row)
	}
	if a, ok := nodes[key]; ok {
		*a = nodeAgg{calls: row.CallCount, errors: row.ErrorCount, latency: percentiles(row.QS)}
	}
}

func consumerKey(row repository.EdgeRow) string {
	return row.Service + "|" + row.ConsumerGroup
}

func buildGraph(rows []repository.EdgeRow, winSecs float64) models.TopologyResponse {
	// Producers go first so each pathway can name its topic's leading producer.
	// Classify by span kind: consumer spans may lack a group (e.g. a client's
	// receive span), which must not make them producers.
	graph := newGraphData(len(rows))
	for _, row := range rows {
		if !row.IsNode && row.Kind == kindProducer {
			graph.addProducer(row, winSecs)
		}
	}
	for _, row := range rows {
		if !row.IsNode && row.Kind == kindConsumer {
			graph.addConsumer(row, winSecs)
		}
	}
	for _, row := range rows {
		if row.IsNode {
			graph.setNode(row)
		}
	}
	for key, calls := range graph.consumeEdges {
		graph.edges = append(graph.edges, models.StreamEdge{
			Source: key[0], Target: key[1], Kind: "consume",
			RatePerSec: float64(calls) / winSecs,
		})
	}
	return models.TopologyResponse{
		Producers: producerNodes(graph.producers, winSecs),
		Topics:    topicNodes(graph.topicProduce, graph.topicProducers, graph.topicGroups, winSecs),
		Consumers: consumerNodes(graph.consumers, graph.consumerMeta, winSecs),
		Edges:     graph.edges, Pathways: graph.pathways,
	}
}

func addSet(m map[string]map[string]struct{}, key, val string) {
	if m[key] == nil {
		m[key] = map[string]struct{}{}
	}
	m[key][val] = struct{}{}
}

func producerNodes(m map[string]*nodeAgg, winSecs float64) []models.ProducerNode {
	out := make([]models.ProducerNode, 0, len(m))
	for svc, a := range m {
		out = append(out, models.ProducerNode{
			Service: svc, RatePerSec: float64(a.calls) / winSecs,
			ErrorRate: metrics.Percentage(a.errors, a.calls),
			P50Ms:     a.latency.p50, P95Ms: a.latency.p95, P99Ms: a.latency.p99,
		})
	}
	slices.SortFunc(out, func(a, b models.ProducerNode) int {
		return cmp.Or(cmp.Compare(b.RatePerSec, a.RatePerSec), strings.Compare(a.Service, b.Service))
	})
	return out
}

func topicNodes(produce map[string]uint64, producers, groups map[string]map[string]struct{}, winSecs float64) []models.TopicNode {
	seen := map[string]struct{}{}
	for t := range produce {
		seen[t] = struct{}{}
	}
	for t := range groups {
		seen[t] = struct{}{}
	}
	out := make([]models.TopicNode, 0, len(seen))
	for t := range seen {
		out = append(out, models.TopicNode{
			Topic: t, RatePerSec: float64(produce[t]) / winSecs,
			ProducerCount: len(producers[t]), ConsumerGroupCount: len(groups[t]),
		})
	}
	slices.SortFunc(out, func(a, b models.TopicNode) int {
		return cmp.Or(cmp.Compare(b.RatePerSec, a.RatePerSec), strings.Compare(a.Topic, b.Topic))
	})
	return out
}

func consumerNodes(m map[string]*nodeAgg, meta map[string][2]string, winSecs float64) []models.ConsumerNode {
	out := make([]models.ConsumerNode, 0, len(m))
	for key, a := range m {
		out = append(out, models.ConsumerNode{
			Service: meta[key][0], Group: meta[key][1],
			RatePerSec: float64(a.calls) / winSecs,
			ErrorRate:  metrics.Percentage(a.errors, a.calls),
			P50Ms:      a.latency.p50, P95Ms: a.latency.p95, P99Ms: a.latency.p99,
		})
	}
	slices.SortFunc(out, func(a, b models.ConsumerNode) int {
		return cmp.Or(cmp.Compare(b.RatePerSec, a.RatePerSec), strings.Compare(a.Service, b.Service), strings.Compare(a.Group, b.Group))
	})
	return out
}
