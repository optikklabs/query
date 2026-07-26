package service

import (
	"math"
	"testing"

	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/infrastructure/repository"
	"github.com/optikklabs/query/internal/modules/infrastructure/seriesdefs"
)

func TestNormalizeUtilization(t *testing.T) {
	cases := []struct {
		in      float64
		wantNil bool
		want    float64
	}{
		{0.5, false, 50},
		{1.0, false, 100},
		{75, false, 75},
		{100.1, true, 0},
		{-0.1, true, 0},
		{math.NaN(), true, 0},
	}
	for _, c := range cases {
		got := normalizeUtilization(c.in)
		if c.wantNil {
			if got != nil {
				t.Errorf("normalizeUtilization(%v) = %v, want nil", c.in, *got)
			}
			continue
		}
		if got == nil || *got != c.want {
			t.Errorf("normalizeUtilization(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestAverageFloats(t *testing.T) {
	if got := averageFloats(nil); got != nil {
		t.Errorf("empty must be nil, got %v", *got)
	}
	if got := averageFloats([]float64{2, 4}); got == nil || *got != 3 {
		t.Errorf("averageFloats([2,4]) = %v, want 3", got)
	}
	if got := averageFloats([]float64{10, 30}); got == nil || *got != 20 {
		t.Errorf("averageFloats([10,30]) = %v, want 20", got)
	}
	// The filtering is what separates it from averageUnfiltered.
	if got := averageFloats([]float64{10, math.NaN(), 30}); got == nil || *got != 20 {
		t.Errorf("averageFloats must skip NaN, got %v", got)
	}
}

func TestAverageUnfiltered(t *testing.T) {
	if got := averageUnfiltered(nil); got != nil {
		t.Errorf("empty must be nil, got %v", *got)
	}
	if got := averageUnfiltered([]float64{10, 30}); got == nil || *got != 20 {
		t.Errorf("averageUnfiltered([10,30]) = %v, want 20", got)
	}
}

// redDerivations computes error% and mean latency, guarding zero requests.
func TestRedDerivations(t *testing.T) {
	if er, al := redDerivations(0, 5, 100); er != 0 || al != 0 {
		t.Errorf("zero requests = (%v,%v), want (0,0)", er, al)
	}
	er, al := redDerivations(4, 1, 200)
	if er != 25 {
		t.Errorf("errorRate = %v, want 25", er)
	}
	if al != 50 {
		t.Errorf("avgLatency = %v, want 50", al)
	}
}

// foldCPUMetricRows normalizes each present CPU metric and averages them; a
// fraction and a percent for the same logical value reconcile to one number.
func TestFoldCPUMetricRows(t *testing.T) {
	rows := []repository.CPUMetricNameRow{
		{MetricName: infraconsts.MetricSystemCPUUtilization, Value: 0.40},
		{MetricName: infraconsts.MetricProcessCPUUsage, Value: 60},
	}
	got := foldCPUMetricRows(rows)
	if got == nil || *got != 50 {
		t.Errorf("fold = %v, want 50 (avg of 40,60)", got)
	}
}

func TestFoldCPUMetricRows_NoneValid(t *testing.T) {
	rows := []repository.CPUMetricNameRow{{MetricName: infraconsts.MetricSystemCPUUsage, Value: 999}}
	if got := foldCPUMetricRows(rows); got != nil {
		t.Errorf("out-of-range only -> nil, got %v", *got)
	}
}

// system memory utilization fraction scales to percent; JVM used/max derives a
// percent; the two average together.
func TestFoldMemoryMetricRows(t *testing.T) {
	rows := []repository.MemoryMetricNameRow{
		{MetricName: infraconsts.MetricSystemMemoryUtilization, Value: 0.40},
		{MetricName: infraconsts.MetricJVMMemoryMax, Value: 200},
		{MetricName: infraconsts.MetricJVMMemoryUsed, Value: 120},
	}
	got := foldMemoryMetricRows(rows)
	if got == nil || *got != 50 {
		t.Errorf("fold = %v, want 50 (avg of 40,60)", got)
	}
}

func TestFoldMemoryMetricRows_NoMaxNoDiv(t *testing.T) {
	rows := []repository.MemoryMetricNameRow{
		{MetricName: infraconsts.MetricJVMMemoryUsed, Value: 120},
		{MetricName: infraconsts.MetricJVMMemoryMax, Value: 0},
	}
	if got := foldMemoryMetricRows(rows); got != nil {
		t.Errorf("zero max -> nil, got %v", *got)
	}
}

func TestHostGroupsForMetricNames(t *testing.T) {
	groups := seriesdefs.Host.GroupsFor([]string{
		infraconsts.MetricSystemNetworkDropped,
		infraconsts.MetricSystemCPUUtilization,
	})
	assertGroups(t, groups, []string{"cpu", "network_errors"})
}

func TestPodGroupsForMetricNames(t *testing.T) {
	groups := seriesdefs.Pod.GroupsFor([]string{
		infraconsts.MetricK8SPodCPUUtilization,
		infraconsts.MetricJVMMemoryUsed,
		"unrelated.metric",
	})
	assertGroups(t, groups, []string{"cpu", "jvm_memory"})
}

func TestSeriesDefForUnknown(t *testing.T) {
	if _, ok := seriesdefs.Pod.Def("nope"); ok {
		t.Fatal("unknown metric id must not resolve")
	}
	if _, ok := seriesdefs.Host.Def("nope"); ok {
		t.Fatal("unknown metric id must not resolve")
	}
}

func assertGroups(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("groups = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("groups = %v, want %v", got, want)
		}
	}
}
