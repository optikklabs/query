package containerdetail

import (
	"testing"

	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
)

func TestFoldREDDerivations(t *testing.T) {
	var out PodOverview
	foldRED(podREDRow{RequestCount: 200, ErrorCount: 10, DurationMsSum: 5000, P95LatencyMs: 120}, &out)

	if out.RequestCount != 200 || out.ErrorCount != 10 {
		t.Fatalf("counts not folded: %+v", out)
	}
	if out.ErrorRate != 5 {
		t.Fatalf("error rate = %v, want 5", out.ErrorRate)
	}
	if out.AvgLatencyMs != 25 {
		t.Fatalf("avg latency = %v, want 25", out.AvgLatencyMs)
	}
	if out.P95LatencyMs != 120 {
		t.Fatalf("p95 = %v, want 120", out.P95LatencyMs)
	}
}

func TestFoldREDNoTraffic(t *testing.T) {
	var out PodOverview
	foldRED(podREDRow{}, &out)

	if out.RequestCount != 0 || out.ErrorRate != 0 || out.AvgLatencyMs != 0 {
		t.Fatalf("zero traffic must fold to zeros: %+v", out)
	}
}

func TestGroupsForMetricNames(t *testing.T) {
	groups := catalog.GroupsFor([]string{
		infraconsts.MetricK8SPodCPUUtilization,
		infraconsts.MetricJVMMemoryUsed,
		"unrelated.metric",
	})
	want := []string{"cpu", "jvm_memory"}
	if len(groups) != len(want) {
		t.Fatalf("groups = %v, want %v", groups, want)
	}
	for i := range want {
		if groups[i] != want[i] {
			t.Fatalf("groups = %v, want %v", groups, want)
		}
	}
}

func TestSeriesDefForUnknown(t *testing.T) {
	if _, ok := catalog.Def("nope"); ok {
		t.Fatal("unknown metric id must not resolve")
	}
}
