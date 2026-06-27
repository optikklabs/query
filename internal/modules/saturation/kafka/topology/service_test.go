package topology

import "testing"

func TestErrRate(t *testing.T) {
	if got := errRate(0, 0); got != 0 {
		t.Errorf("errRate(0,0) = %v, want 0 (no div-by-zero)", got)
	}
	if got := errRate(5, 0); got != 0 {
		t.Errorf("errRate(5,0) = %v, want 0", got)
	}
	if got := errRate(3, 12); got != 0.25 {
		t.Errorf("errRate(3,12) = %v, want 0.25", got)
	}
}

func TestP95(t *testing.T) {
	if got := p95(nil); got != 0 {
		t.Errorf("p95(nil) = %v, want 0", got)
	}
	if got := p95([]float64{1}); got != 0 {
		t.Errorf("p95(len1) = %v, want 0", got)
	}
	if got := p95([]float64{1, 5, 9}); got != 5 {
		t.Errorf("p95([1,5,9]) = %v, want 5 (index 1)", got)
	}
}

// Flow: produce+consume edge rows -> the full topology graph. Pins rate=calls/
// winSecs, set-dedup counts, worst-topic p95, service|group consumer keying,
// edge fan-out, and pathway linkage to the top producer.
func TestBuildGraph_Flow(t *testing.T) {
	const winSecs = 10.0
	produce := []produceEdgeRow{
		{Service: "svcA", Topic: "orders", CallCount: 100, ErrorCount: 10, QS: []float64{1, 5, 9}},
		{Service: "svcA", Topic: "payments", CallCount: 50, ErrorCount: 0, QS: []float64{2, 8, 12}},
		{Service: "svcB", Topic: "orders", CallCount: 20, ErrorCount: 0, QS: []float64{1, 3, 4}},
	}
	consume := []consumeEdgeRow{
		{Service: "consumer1", Topic: "orders", ConsumerGroup: "g1", CallCount: 80, ErrorCount: 4, QS: []float64{1, 6, 10}},
		{Service: "consumer1", Topic: "payments", ConsumerGroup: "g1", CallCount: 40, ErrorCount: 0, QS: []float64{1, 2, 3}},
		{Service: "consumer2", Topic: "orders", ConsumerGroup: "g2", CallCount: 10, ErrorCount: 0, QS: []float64{1, 1, 1}},
	}

	g := buildGraph(produce, consume, winSecs)

	if len(g.Producers) != 2 {
		t.Fatalf("got %d producers, want 2: %+v", len(g.Producers), g.Producers)
	}
	pa := g.Producers[0]
	if pa.Service != "svcA" || pa.RatePerSec != 15 || pa.P95Ms != 8 || pa.ErrorRate != 10.0/150.0 {
		t.Errorf("producer svcA = %+v, want rate 15, p95 8, errRate 10/150", pa)
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
	if c := cons["consumer1|g1"]; c.RatePerSec != 12 || c.P95Ms != 6 || c.ErrorRate != 4.0/120.0 {
		t.Errorf("consumer1|g1 = %+v, want rate 12, p95 6, errRate 4/120", c)
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
			if pw.ConsumeRatePerSec != 8 || pw.ProduceRatePerSec != 12 || pw.ErrorRate != 4.0/80.0 {
				t.Errorf("orders/g1 pathway = %+v, want consume 8, produce 12, errRate 4/80", pw)
			}
		}
	}
}

func TestBuildGraph_Empty(t *testing.T) {
	g := buildGraph(nil, nil, 1)
	if len(g.Producers) != 0 || len(g.Topics) != 0 || len(g.Consumers) != 0 || len(g.Edges) != 0 || len(g.Pathways) != 0 {
		t.Errorf("empty input should yield empty graph, got %+v", g)
	}
}
