package topology

import "testing"

// classifyHealth uses strict > thresholds: degraded above 0.01, unhealthy above
// 0.05; boundary values fall to the lower severity.
func TestClassifyHealth(t *testing.T) {
	cases := []struct {
		errRate float64
		want    string
	}{
		{0, HealthHealthy},
		{degradedErrorRate, HealthHealthy},
		{2, HealthDegraded},
		{unhealthyErrorRate, HealthDegraded},
		{6, HealthUnhealthy},
	}
	for _, c := range cases {
		if got := classifyHealth(c.errRate); got != c.want {
			t.Errorf("classifyHealth(%v) = %q, want %q", c.errRate, got, c.want)
		}
	}
}

func TestBuildGraph_Nodes(t *testing.T) {
	g := BuildGraph([]NodeAgg{
		{Service: "a", RequestCount: 100, ErrorCount: 10, P50Ms: 1, P95Ms: 5, P99Ms: 9},
		{Service: "b", RequestCount: 0, ErrorCount: 0},
	}, nil)

	if len(g.Nodes) != 2 {
		t.Fatalf("got %d nodes, want 2", len(g.Nodes))
	}
	a := g.Nodes[0]
	if a.ErrorRate != 10 || a.Health != HealthUnhealthy || a.P95LatencyMs != 5 {
		t.Errorf("node a = %+v, want error rate 10%%, unhealthy, p95 5", a)
	}
	if b := g.Nodes[1]; b.ErrorRate != 0 || b.Health != HealthHealthy {
		t.Errorf("node b = %+v, want errRate 0 (no div-by-zero), healthy", b)
	}
}

func TestBuildGraph_AddsMissingEdgeNodes(t *testing.T) {
	g := BuildGraph(
		[]NodeAgg{{Service: "a", RequestCount: 10}},
		[]EdgeAgg{{Source: "a", Target: "b", CallCount: 5, ErrorCount: 1}},
	)

	names := map[string]string{}
	for _, n := range g.Nodes {
		names[n.Name] = n.Health
	}
	if _, ok := names["b"]; !ok {
		t.Fatalf("missing edge target 'b' not synthesized: %+v", g.Nodes)
	}
	if names["b"] != HealthHealthy {
		t.Errorf("synthesized node health = %q, want healthy", names["b"])
	}
	if len(g.Edges) != 1 || g.Edges[0].ErrorRate != 20 {
		t.Errorf("edge = %+v, want error rate 20%%", g.Edges)
	}
}

func TestBuildGraph_Empty(t *testing.T) {
	g := BuildGraph(nil, nil)
	if g.Nodes == nil || g.Edges == nil {
		t.Errorf("slices must be non-nil for JSON encoding: %+v", g)
	}
	if len(g.Nodes) != 0 || len(g.Edges) != 0 {
		t.Errorf("want empty graph, got %+v", g)
	}
}
