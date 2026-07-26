package service

import (
	"context"
	"sort"

	"github.com/optikklabs/query/internal/modules/saturation/kafka/models"
	"github.com/optikklabs/query/internal/modules/saturation/kafka/repository"
	"github.com/optikklabs/query/internal/shared/metrics"
)

// GetClients lists the tenant's Kafka clients. It is separate from the graph so
// the picker stays complete while the graph stays scoped.
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

// GetTopology builds the producers->topics->consumers graph for the given
// services: their topics and the peers on the far side of them.
func (s *Service) GetTopology(ctx context.Context, tenantID, startMs, endMs int64, services []string) (models.TopologyResponse, error) {
	if len(services) == 0 {
		return buildGraph(nil, 1), nil
	}
	rows, err := s.repo.QueryEdges(ctx, tenantID, startMs, endMs, services)
	if err != nil {
		return models.TopologyResponse{}, err
	}

	winSecs := float64(endMs-startMs) / 1000
	if winSecs <= 0 {
		winSecs = 1
	}
	return buildGraph(rows, winSecs), nil
}

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

func errRate(errors, calls uint64) float64 {
	return metrics.Percentage(errors, calls)
}

type nodeAgg struct {
	calls   uint64
	errors  uint64
	latency percentileValues
}

// buildGraph folds the edge rows into the graph. Produce rows must be applied
// first: pathways reference each topic's top producer, which only the produce
// pass knows.
func buildGraph(rows []repository.EdgeRow, winSecs float64) models.TopologyResponse {
	producers := map[string]*nodeAgg{}
	consumers := map[string]*nodeAgg{}
	consumerMeta := map[string][2]string{}
	topicProduce := map[string]uint64{}
	topicProducers := map[string]map[string]struct{}{}
	topicGroups := map[string]map[string]struct{}{}
	topicTop := map[string]struct {
		svc   string
		calls uint64
	}{}
	edges := make([]models.StreamEdge, 0, len(rows))

	for _, row := range rows {
		if row.ConsumerGroup != "" {
			continue
		}
		acc(producers, row.Service, row.CallCount, row.ErrorCount, percentiles(row.QS))
		topicProduce[row.Topic] += row.CallCount
		addSet(topicProducers, row.Topic, row.Service)
		top, exists := topicTop[row.Topic]
		if !exists || row.CallCount > top.calls || (row.CallCount == top.calls && row.Service < top.svc) {
			topicTop[row.Topic] = struct {
				svc   string
				calls uint64
			}{row.Service, row.CallCount}
		}
		edges = append(edges, models.StreamEdge{
			Source: row.Service, Target: row.Topic, Kind: "produce",
			RatePerSec: float64(row.CallCount) / winSecs,
		})
	}

	consumeEdge := map[[2]string]uint64{}
	pathways := make([]models.Pathway, 0, len(rows))
	for _, row := range rows {
		if row.ConsumerGroup == "" {
			continue
		}
		key := row.Service + "|" + row.ConsumerGroup
		acc(consumers, key, row.CallCount, row.ErrorCount, percentiles(row.QS))
		consumerMeta[key] = [2]string{row.Service, row.ConsumerGroup}
		addSet(topicGroups, row.Topic, row.ConsumerGroup)
		consumeEdge[[2]string{row.Topic, row.Service}] += row.CallCount
		pathways = append(pathways, models.Pathway{
			Producer: topicTop[row.Topic].svc, Topic: row.Topic,
			Group: row.ConsumerGroup, Consumer: row.Service,
			ProduceRatePerSec: float64(topicProduce[row.Topic]) / winSecs,
			ConsumeRatePerSec: float64(row.CallCount) / winSecs,
			ErrorRate:         errRate(row.ErrorCount, row.CallCount),
		})
	}
	for k, calls := range consumeEdge {
		edges = append(edges, models.StreamEdge{
			Source: k[0], Target: k[1], Kind: "consume",
			RatePerSec: float64(calls) / winSecs,
		})
	}

	return models.TopologyResponse{
		Producers: producerNodes(producers, winSecs),
		Topics:    topicNodes(topicProduce, topicProducers, topicGroups, winSecs),
		Consumers: consumerNodes(consumers, consumerMeta, winSecs),
		Edges:     edges,
		Pathways:  pathways,
	}
}

func acc(m map[string]*nodeAgg, key string, calls, errors uint64, latency percentileValues) {
	a := m[key]
	if a == nil {
		a = &nodeAgg{}
		m[key] = a
	}
	a.calls += calls
	a.errors += errors
	a.latency.p50 = max(a.latency.p50, latency.p50)
	a.latency.p95 = max(a.latency.p95, latency.p95)
	a.latency.p99 = max(a.latency.p99, latency.p99)
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
			ErrorRate: errRate(a.errors, a.calls),
			P50Ms:     a.latency.p50, P95Ms: a.latency.p95, P99Ms: a.latency.p99,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RatePerSec != out[j].RatePerSec {
			return out[i].RatePerSec > out[j].RatePerSec
		}
		return out[i].Service < out[j].Service
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
	sort.Slice(out, func(i, j int) bool {
		if out[i].RatePerSec != out[j].RatePerSec {
			return out[i].RatePerSec > out[j].RatePerSec
		}
		return out[i].Topic < out[j].Topic
	})
	return out
}

func consumerNodes(m map[string]*nodeAgg, meta map[string][2]string, winSecs float64) []models.ConsumerNode {
	out := make([]models.ConsumerNode, 0, len(m))
	for key, a := range m {
		out = append(out, models.ConsumerNode{
			Service: meta[key][0], Group: meta[key][1],
			RatePerSec: float64(a.calls) / winSecs,
			ErrorRate:  errRate(a.errors, a.calls),
			P50Ms:      a.latency.p50, P95Ms: a.latency.p95, P99Ms: a.latency.p99,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RatePerSec != out[j].RatePerSec {
			return out[i].RatePerSec > out[j].RatePerSec
		}
		if out[i].Service != out[j].Service {
			return out[i].Service < out[j].Service
		}
		return out[i].Group < out[j].Group
	})
	return out
}
