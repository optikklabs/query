package cloud

import "testing"

// CategoryFor maps known platforms and defaults unknowns to "other".
func TestCategoryFor(t *testing.T) {
	cases := map[string]string{
		"aws_eks":               CategoryCompute,
		"gcp_cloud_sql":         CategoryData,
		"azure_blob_storage":    CategoryStorage,
		"aws_msk":               CategoryStreaming,
		"gcp_vertex_ai":         CategoryAI,
		"gcp_kubernetes_engine": CategoryCompute,
		"something_unknown":     CategoryOther,
		"":                      CategoryOther,
	}
	for platform, want := range cases {
		if got := CategoryFor(platform); got != want {
			t.Errorf("CategoryFor(%q) = %q, want %q", platform, got, want)
		}
	}
}

// classifyHealth uses the same thresholds as the nodes module.
func TestClassifyHealth(t *testing.T) {
	cases := map[float64]string{
		0:    "healthy",
		2:    "healthy",
		2.01: "degraded",
		10:   "degraded",
		10.1: "unhealthy",
	}
	for rate, want := range cases {
		if got := classifyHealth(rate); got != want {
			t.Errorf("classifyHealth(%v) = %q, want %q", rate, got, want)
		}
	}
}

// redDerivations guards zero requests and computes error% + mean latency.
func TestRedDerivations(t *testing.T) {
	if er, al := redDerivations(0, 5, 100); er != 0 || al != 0 {
		t.Errorf("zero requests = (%v,%v), want (0,0)", er, al)
	}
	er, al := redDerivations(8, 2, 400)
	if er != 25 {
		t.Errorf("errorRate = %v, want 25", er)
	}
	if al != 50 {
		t.Errorf("avgLatency = %v, want 50", al)
	}
}

// aggregateCategories folds platforms into ordered category buckets per provider.
func TestAggregateCategories(t *testing.T) {
	rows := []CategoryRow{
		{Provider: "aws", Platform: "aws_ec2", Count: 10},
		{Provider: "aws", Platform: "aws_eks", Count: 5},       // compute again -> 15
		{Provider: "aws", Platform: "aws_rds", Count: 3},       // data
		{Provider: "aws", Platform: "mystery_thing", Count: 2}, // other
	}
	got := aggregateCategories(rows)
	aws := got["aws"]
	if len(aws) != 3 {
		t.Fatalf("aws buckets = %d, want 3", len(aws))
	}
	// categoryOrder => compute first, then data, then other.
	if aws[0].Category != CategoryCompute || aws[0].Count != 15 {
		t.Errorf("bucket[0] = %+v, want compute=15", aws[0])
	}
	if aws[1].Category != CategoryData || aws[1].Count != 3 {
		t.Errorf("bucket[1] = %+v, want data=3", aws[1])
	}
	if aws[2].Category != CategoryOther || aws[2].Count != 2 {
		t.Errorf("bucket[2] = %+v, want other=2", aws[2])
	}
}

// aggregateHealth classifies each entity and tallies per provider.
func TestAggregateHealth(t *testing.T) {
	rows := []HealthRow{
		{Provider: "aws", Entity: "a", RequestCount: 100, ErrorCount: 0},  // healthy
		{Provider: "aws", Entity: "b", RequestCount: 100, ErrorCount: 5},  // degraded (5%)
		{Provider: "aws", Entity: "c", RequestCount: 100, ErrorCount: 20}, // unhealthy (20%)
		{Provider: "gcp", Entity: "d", RequestCount: 0, ErrorCount: 0},    // healthy (guard)
	}
	got := aggregateHealth(rows)
	if got["aws"] != (HealthCounts{Healthy: 1, Degraded: 1, Unhealthy: 1}) {
		t.Errorf("aws health = %+v", got["aws"])
	}
	if got["gcp"] != (HealthCounts{Healthy: 1}) {
		t.Errorf("gcp health = %+v", got["gcp"])
	}
}
