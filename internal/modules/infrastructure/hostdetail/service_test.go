package hostdetail

import (
	"testing"

	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
)

func TestFoldKPIsPerStateAgent(t *testing.T) {
	rows := []kpiRow{
		{MetricName: infraconsts.MetricSystemCPUUtilization, State: "idle", Value: 0.6},
		{MetricName: infraconsts.MetricSystemCPUUtilization, State: "user", Value: 0.3},
		{MetricName: infraconsts.MetricSystemMemoryUtilization, State: "used", Value: 0.42},
		{MetricName: infraconsts.MetricSystemFilesystemUtil, Mount: "/", Value: 0.35},
		{MetricName: infraconsts.MetricSystemFilesystemUtil, Mount: "/data", Value: 0.9},
		{MetricName: infraconsts.MetricSystemCPULoadAvg1m, Value: 1.5},
	}
	var out HostOverview
	foldKPIs(rows, &out)

	assertPct(t, "cpu", out.CPUPct, 40)
	assertPct(t, "memory", out.MemoryPct, 42)
	assertPct(t, "disk", out.DiskPct, 90)
	assertPct(t, "load1m", out.Load1m, 1.5)
	if out.Load5m != nil || out.ProcessCount != nil {
		t.Fatalf("unreported KPIs must stay nil")
	}
}

func TestFoldKPIsPlainUtilizationFallback(t *testing.T) {
	rows := []kpiRow{
		{MetricName: infraconsts.MetricSystemCPUUtilization, State: "", Value: 0.25},
		{MetricName: infraconsts.MetricSystemMemoryUtilization, State: "", Value: 55},
	}
	var out HostOverview
	foldKPIs(rows, &out)

	assertPct(t, "cpu", out.CPUPct, 25)
	// Values already >1 pass through as percentages.
	assertPct(t, "memory", out.MemoryPct, 55)
}

func TestFoldKPIsSkipsInvalidValues(t *testing.T) {
	rows := []kpiRow{
		{MetricName: infraconsts.MetricSystemCPULoadAvg1m, Value: -1},
	}
	var out HostOverview
	foldKPIs(rows, &out)
	if out.Load1m != nil {
		t.Fatalf("negative values must be skipped")
	}
}

func TestGroupsForMetricNames(t *testing.T) {
	groups := catalog.GroupsFor([]string{
		infraconsts.MetricSystemNetworkDropped,
		infraconsts.MetricSystemCPUUtilization,
	})
	want := []string{"cpu", "network_errors"}
	if len(groups) != len(want) {
		t.Fatalf("groups = %v, want %v", groups, want)
	}
	for i := range want {
		if groups[i] != want[i] {
			t.Fatalf("groups = %v, want %v", groups, want)
		}
	}
}

func assertPct(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s: got nil, want %v", name, want)
	}
	diff := *got - want
	if diff < -0.0001 || diff > 0.0001 {
		t.Fatalf("%s: got %v, want %v", name, *got, want)
	}
}

func TestAboutFromMeta(t *testing.T) {
	if aboutFromMeta(hostMetaRow{}) != nil {
		t.Fatal("no attrs must yield nil About")
	}
	about := aboutFromMeta(hostMetaRow{OSType: "linux", CloudProvider: "gcp"})
	if about == nil || about.OSType != "linux" || about.CloudProvider != "gcp" {
		t.Fatalf("about = %+v", about)
	}
}
