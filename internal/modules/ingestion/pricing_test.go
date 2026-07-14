package ingestion

import (
	"math"
	"testing"
)

var testRates = Rates{Currency: "USD", PerGBLogsTraces: 0.10, PerDPMMetrics: 0.008}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestEstimateCostVolumeAndDPM(t *testing.T) {
	// 10 GB logs, 5 GB traces over 15 days of a 30-day month; 60,000 metric
	// datapoints over a 10-minute window -> 6,000 DPM.
	u := usageQuantities{
		logsBytes:   10 * bytesPerGB,
		spansBytes:  5 * bytesPerGB,
		metricDPs:   60_000,
		windowMin:   10,
		daysElapsed: 15,
		daysInMonth: 30,
	}
	got := estimateCost(u, testRates)

	if got.Currency != "USD" || len(got.Lines) != 3 {
		t.Fatalf("unexpected shape: %+v", got)
	}
	logs, traces, metrics := got.Lines[0], got.Lines[1], got.Lines[2]

	if !approx(logs.Cost, 1.0) { // 10 GB * 0.10
		t.Errorf("logs cost: want 1.0, got %v", logs.Cost)
	}
	if !approx(logs.ProjectedCost, 2.0) { // 15->30 days doubles
		t.Errorf("logs projected: want 2.0, got %v", logs.ProjectedCost)
	}
	if !approx(traces.Cost, 0.5) {
		t.Errorf("traces cost: want 0.5, got %v", traces.Cost)
	}
	if !approx(metrics.Quantity, 6000) {
		t.Errorf("dpm: want 6000, got %v", metrics.Quantity)
	}
	if !approx(metrics.Cost, 48.0) { // 6000 DPM * 0.008
		t.Errorf("metrics cost: want 48.0, got %v", metrics.Cost)
	}
	// DPM is a rate: current == projected.
	if !approx(metrics.Cost, metrics.ProjectedCost) {
		t.Errorf("metrics projection should be flat: %v vs %v", metrics.Cost, metrics.ProjectedCost)
	}
	if !approx(got.CurrentCost, 1.0+0.5+48.0) {
		t.Errorf("current total: want 49.5, got %v", got.CurrentCost)
	}
	if !approx(got.ProjectedMonthlyCost, 2.0+1.0+48.0) {
		t.Errorf("projected total: want 51.0, got %v", got.ProjectedMonthlyCost)
	}
}

func TestEstimateCostZeroWindowAndDaysAreSafe(t *testing.T) {
	got := estimateCost(usageQuantities{metricDPs: 100}, testRates)
	if got.CurrentCost != 0 || got.ProjectedMonthlyCost != 0 {
		t.Errorf("zero window/days must not divide by zero: %+v", got)
	}
}
