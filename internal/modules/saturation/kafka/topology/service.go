package topology

import (
	"context"
	"sort"

	"github.com/optikklabs/query/internal/shared/metrics"
	"golang.org/x/sync/errgroup"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// GetTopology builds the producers->topics->consumers graph for the window.
func (s *Service) GetTopology(ctx context.Context, tenantID, startMs, endMs int64, _ string) (TopologyResponse, error) {
	var (
		produceRows []produceEdgeRow
		consumeRows []consumeEdgeRow
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		rows, err := s.repo.QueryProduceEdges(gctx, tenantID, startMs, endMs)
		produceRows = rows
		return err
	})
	g.Go(func() error {
		rows, err := s.repo.QueryConsumeEdges(gctx, tenantID, startMs, endMs)
		consumeRows = rows
		return err
	})
	if err := g.Wait(); err != nil {
		return TopologyResponse{}, err
	}

	winSecs := float64(endMs-startMs) / 1000
	if winSecs <= 0 {
		winSecs = 1
	}
	return buildGraph(produceRows, consumeRows, winSecs), nil
}

func p95(qs []float64) float64 {
	if len(qs) >= 2 {
		return qs[1]
	}
	return 0
}

func errRate(errors, calls uint64) float64 {
	return metrics.Percentage(errors, calls)
}

type nodeAgg struct {
	calls  uint64
	errors uint64
	maxP95 float64
}

func buildGraph(produceRows []produceEdgeRow, consumeRows []consumeEdgeRow, winSecs float64) TopologyResponse {
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
	edges := make([]StreamEdge, 0, len(produceRows)+len(consumeRows))

	for _, row := range produceRows {
		acc(producers, row.Service, row.CallCount, row.ErrorCount, p95(row.QS))
		topicProduce[row.Topic] += row.CallCount
		addSet(topicProducers, row.Topic, row.Service)
		if row.CallCount >= topicTop[row.Topic].calls {
			topicTop[row.Topic] = struct {
				svc   string
				calls uint64
			}{row.Service, row.CallCount}
		}
		edges = append(edges, StreamEdge{
			Source: row.Service, Target: row.Topic, Kind: "produce",
			RatePerSec: float64(row.CallCount) / winSecs,
		})
	}

	consumeEdge := map[[2]string]uint64{}
	pathways := make([]Pathway, 0, len(consumeRows))
	for _, row := range consumeRows {
		key := row.Service + "|" + row.ConsumerGroup
		acc(consumers, key, row.CallCount, row.ErrorCount, p95(row.QS))
		consumerMeta[key] = [2]string{row.Service, row.ConsumerGroup}
		addSet(topicGroups, row.Topic, row.ConsumerGroup)
		consumeEdge[[2]string{row.Topic, row.Service}] += row.CallCount
		pathways = append(pathways, Pathway{
			Producer: topicTop[row.Topic].svc, Topic: row.Topic,
			Group: row.ConsumerGroup, Consumer: row.Service,
			ProduceRatePerSec: float64(topicProduce[row.Topic]) / winSecs,
			ConsumeRatePerSec: float64(row.CallCount) / winSecs,
			ErrorRate:         errRate(row.ErrorCount, row.CallCount),
		})
	}
	for k, calls := range consumeEdge {
		edges = append(edges, StreamEdge{
			Source: k[0], Target: k[1], Kind: "consume",
			RatePerSec: float64(calls) / winSecs,
		})
	}

	return TopologyResponse{
		Producers: producerNodes(producers, winSecs),
		Topics:    topicNodes(topicProduce, topicProducers, topicGroups, winSecs),
		Consumers: consumerNodes(consumers, consumerMeta, winSecs),
		Edges:     edges,
		Pathways:  pathways,
	}
}

func acc(m map[string]*nodeAgg, key string, calls, errors uint64, p float64) {
	a := m[key]
	if a == nil {
		a = &nodeAgg{}
		m[key] = a
	}
	a.calls += calls
	a.errors += errors
	if p > a.maxP95 {
		a.maxP95 = p
	}
}

func addSet(m map[string]map[string]struct{}, key, val string) {
	if m[key] == nil {
		m[key] = map[string]struct{}{}
	}
	m[key][val] = struct{}{}
}

func producerNodes(m map[string]*nodeAgg, winSecs float64) []ProducerNode {
	out := make([]ProducerNode, 0, len(m))
	for svc, a := range m {
		out = append(out, ProducerNode{
			Service: svc, RatePerSec: float64(a.calls) / winSecs,
			ErrorRate: errRate(a.errors, a.calls), P95Ms: a.maxP95,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RatePerSec > out[j].RatePerSec })
	return out
}

func topicNodes(produce map[string]uint64, producers, groups map[string]map[string]struct{}, winSecs float64) []TopicNode {
	seen := map[string]struct{}{}
	for t := range produce {
		seen[t] = struct{}{}
	}
	for t := range groups {
		seen[t] = struct{}{}
	}
	out := make([]TopicNode, 0, len(seen))
	for t := range seen {
		out = append(out, TopicNode{
			Topic: t, RatePerSec: float64(produce[t]) / winSecs,
			ProducerCount: len(producers[t]), ConsumerGroupCount: len(groups[t]),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RatePerSec > out[j].RatePerSec })
	return out
}

func consumerNodes(m map[string]*nodeAgg, meta map[string][2]string, winSecs float64) []ConsumerNode {
	out := make([]ConsumerNode, 0, len(m))
	for key, a := range m {
		out = append(out, ConsumerNode{
			Service: meta[key][0], Group: meta[key][1],
			RatePerSec: float64(a.calls) / winSecs,
			ErrorRate:  errRate(a.errors, a.calls), P95Ms: a.maxP95,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RatePerSec > out[j].RatePerSec })
	return out
}
