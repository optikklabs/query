package service

import (
	"testing"

	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
	"github.com/optikklabs/query/internal/modules/infrastructure/models"
	"github.com/optikklabs/query/internal/modules/infrastructure/repository"
)

func TestFoldKPIsPerStateAgent(t *testing.T) {
	rows := []repository.KPIRow{
		{MetricName: infraconsts.MetricSystemCPUUtilization, State: "idle", Value: 0.6},
		{MetricName: infraconsts.MetricSystemCPUUtilization, State: "user", Value: 0.3},
		{MetricName: infraconsts.MetricSystemMemoryUtilization, State: "used", Value: 0.42},
		{MetricName: infraconsts.MetricSystemFilesystemUtil, Mount: "/", Value: 0.35},
		{MetricName: infraconsts.MetricSystemFilesystemUtil, Mount: "/data", Value: 0.9},
		{MetricName: infraconsts.MetricSystemCPULoadAvg1m, Value: 1.5},
	}
	var out models.HostOverview
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
	rows := []repository.KPIRow{
		{MetricName: infraconsts.MetricSystemCPUUtilization, State: "", Value: 0.25},
		{MetricName: infraconsts.MetricSystemMemoryUtilization, State: "", Value: 55},
	}
	var out models.HostOverview
	foldKPIs(rows, &out)

	assertPct(t, "cpu", out.CPUPct, 25)
	// Values already >1 pass through as percentages.
	assertPct(t, "memory", out.MemoryPct, 55)
}

func TestFoldKPIsSkipsInvalidValues(t *testing.T) {
	rows := []repository.KPIRow{
		{MetricName: infraconsts.MetricSystemCPULoadAvg1m, Value: -1},
	}
	var out models.HostOverview
	foldKPIs(rows, &out)
	if out.Load1m != nil {
		t.Fatalf("negative values must be skipped")
	}
}

func TestAboutFromMeta(t *testing.T) {
	if aboutFromMeta(repository.HostMetaRow{}) != nil {
		t.Fatal("no attrs must yield nil About")
	}
	about := aboutFromMeta(repository.HostMetaRow{OSType: "linux", CloudProvider: "gcp"})
	if about == nil || about.OSType != "linux" || about.CloudProvider != "gcp" {
		t.Fatalf("about = %+v", about)
	}
}

func TestFoldREDDerivations(t *testing.T) {
	var out models.PodOverview
	foldRED(repository.PodREDRow{RequestCount: 200, ErrorCount: 10, DurationMsSum: 5000, P95LatencyMs: 120}, &out)

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
	var out models.PodOverview
	foldRED(repository.PodREDRow{}, &out)

	if out.RequestCount != 0 || out.ErrorRate != 0 || out.AvgLatencyMs != 0 {
		t.Fatalf("zero traffic must fold to zeros: %+v", out)
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
