package topology

import (
	"strings"
	"testing"
)

func TestErrRate(t *testing.T) {
	if got := errRate(0, 0); got != 0 {
		t.Errorf("errRate(0,0) = %v, want 0 (no div-by-zero)", got)
	}
	if got := errRate(5, 0); got != 0 {
		t.Errorf("errRate(5,0) = %v, want 0", got)
	}
	if got := errRate(3, 12); got != 25 {
		t.Errorf("errRate(3,12) = %v, want 25", got)
	}
}

func TestPercentiles(t *testing.T) {
	tests := []struct {
		name string
		qs   []float64
		want percentileValues
	}{
		{name: "empty", want: percentileValues{}},
		{name: "p50 only", qs: []float64{1}, want: percentileValues{p50: 1}},
		{name: "p50 and p95", qs: []float64{1, 5}, want: percentileValues{p50: 1, p95: 5}},
		{name: "all", qs: []float64{1, 5, 9}, want: percentileValues{p50: 1, p95: 5, p99: 9}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := percentiles(tt.qs); got != tt.want {
				t.Fatalf("percentiles(%v) = %+v, want %+v", tt.qs, got, tt.want)
			}
		})
	}
}

// Flow: produce+consume edge rows -> the full topology graph. Pins rate=calls/
// winSecs, set-dedup counts, worst-topic p95, service|group consumer keying,
// edge fan-out, and pathway linkage to the top producer.
func TestBuildGraph_Flow(t *testing.T) {
	const winSecs = 10.0
	// Interleaved on purpose: an empty consumer group is what marks a produce
	// row, so the split must not depend on the rows arriving grouped by side.
	rows := []edgeRow{
		{Service: "svcA", Topic: "orders", CallCount: 100, ErrorCount: 10, QS: []float64{1, 5, 9}},
		{Service: "consumer1", Topic: "orders", ConsumerGroup: "g1", CallCount: 80, ErrorCount: 4, QS: []float64{1, 6, 10}},
		{Service: "svcA", Topic: "payments", CallCount: 50, ErrorCount: 0, QS: []float64{2, 8, 12}},
		{Service: "consumer1", Topic: "payments", ConsumerGroup: "g1", CallCount: 40, ErrorCount: 0, QS: []float64{1, 2, 3}},
		{Service: "svcB", Topic: "orders", CallCount: 20, ErrorCount: 0, QS: []float64{1, 3, 4}},
		{Service: "consumer2", Topic: "orders", ConsumerGroup: "g2", CallCount: 10, ErrorCount: 0, QS: []float64{1, 1, 1}},
	}

	g := buildGraph(rows, winSecs)

	if len(g.Producers) != 2 {
		t.Fatalf("got %d producers, want 2: %+v", len(g.Producers), g.Producers)
	}
	pa := g.Producers[0]
	if pa.Service != "svcA" || pa.RatePerSec != 15 || pa.P50Ms != 2 || pa.P95Ms != 8 || pa.P99Ms != 12 || pa.ErrorRate != 1000.0/150.0 {
		t.Errorf("producer svcA = %+v, want rate 15, p50/p95/p99 2/8/12, error rate 10/150", pa)
	}
	if g.Producers[1].Service != "svcB" || g.Producers[1].RatePerSec != 2 {
		t.Errorf("producer[1] = %+v, want svcB rate 2 (sorted)", g.Producers[1])
	}

	topics := map[string]TopicNode{}
	for _, tn := range g.Topics {
		topics[tn.Topic] = tn
	}
	if o := topics["orders"]; o.RatePerSec != 12 || o.ProducerCount != 2 || o.ConsumerGroupCount != 2 {
		t.Errorf("orders topic = %+v, want rate 12, producers 2, groups 2", o)
	}
	if p := topics["payments"]; p.RatePerSec != 5 || p.ProducerCount != 1 || p.ConsumerGroupCount != 1 {
		t.Errorf("payments topic = %+v, want rate 5, producers 1, groups 1", p)
	}

	cons := map[string]ConsumerNode{}
	for _, cn := range g.Consumers {
		cons[cn.Service+"|"+cn.Group] = cn
	}
	if c := cons["consumer1|g1"]; c.RatePerSec != 12 || c.P50Ms != 1 || c.P95Ms != 6 || c.P99Ms != 10 || c.ErrorRate != 400.0/120.0 {
		t.Errorf("consumer1|g1 = %+v, want rate 12, p50/p95/p99 1/6/10, error rate 4/120", c)
	}
	if c := cons["consumer2|g2"]; c.RatePerSec != 1 {
		t.Errorf("consumer2|g2 = %+v, want rate 1", c)
	}

	var prod, cons2 int
	for _, e := range g.Edges {
		switch e.Kind {
		case "produce":
			prod++
		case "consume":
			cons2++
		}
	}
	if prod != 3 || cons2 != 3 {
		t.Errorf("edges produce=%d consume=%d, want 3/3", prod, cons2)
	}

	if len(g.Pathways) != 3 {
		t.Fatalf("got %d pathways, want 3: %+v", len(g.Pathways), g.Pathways)
	}
	for _, pw := range g.Pathways {
		if pw.Topic == "orders" && pw.Producer != "svcA" {
			t.Errorf("orders pathway producer = %q, want svcA (top producer)", pw.Producer)
		}
		if pw.Group == "g1" && pw.Topic == "orders" {
			if pw.ConsumeRatePerSec != 8 || pw.ProduceRatePerSec != 12 || pw.ErrorRate != 400.0/80.0 {
				t.Errorf("orders/g1 pathway = %+v, want consume 8, produce 12, error rate 4/80", pw)
			}
		}
	}
}

func TestBuildGraph_TopProducerTieIsDeterministic(t *testing.T) {
	rows := []edgeRow{
		{Service: "z-service", Topic: "orders", CallCount: 10},
		{Service: "a-service", Topic: "orders", CallCount: 10},
		{Service: "consumer", Topic: "orders", ConsumerGroup: "group", CallCount: 5},
	}

	graph := buildGraph(rows, 1)
	if got := graph.Pathways[0].Producer; got != "a-service" {
		t.Fatalf("top producer = %q, want deterministic lexical tie-breaker", got)
	}
}

func TestQueriesDoNotSortByTraffic(t *testing.T) {
	if strings.Contains(clientsQuery, "count()") {
		t.Fatal("clients query must not rank services by series count")
	}
	if !strings.Contains(clientsQuery, "SELECT DISTINCT service") || !strings.Contains(clientsQuery, "ORDER BY service") {
		t.Fatal("clients query must return distinct services in deterministic name order")
	}
	if strings.Contains(edgesQuery("rollup"), "ORDER BY call_count") {
		t.Fatal("edges query must not sort aggregated rows by call count")
	}
}

func TestBuildGraph_Empty(t *testing.T) {
	g := buildGraph(nil, 1)
	if len(g.Producers) != 0 || len(g.Topics) != 0 || len(g.Consumers) != 0 || len(g.Edges) != 0 || len(g.Pathways) != 0 {
		t.Errorf("empty input should yield empty graph, got %+v", g)
	}
}

// An empty or comma-padded param must not widen the query to every service.
func TestParseServices(t *testing.T) {
	cases := map[string][]string{
		"":          {},
		",":         {},
		"  ":        {},
		"a":         {"a"},
		"a,b":       {"a", "b"},
		" a , ,b, ": {"a", "b"},
	}
	for raw, want := range cases {
		got := parseServices(raw)
		if len(got) != len(want) {
			t.Errorf("parseServices(%q) = %v, want %v", raw, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("parseServices(%q) = %v, want %v", raw, got, want)
				break
			}
		}
	}
}
