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
	if !approx(traces.Cost, 0.5) {
		t.Errorf("traces cost: want 0.5, got %v", traces.Cost)
	}
	if !approx(metrics.Quantity, 6000) {
		t.Errorf("dpm: want 6000, got %v", metrics.Quantity)
	}
	if !approx(metrics.Cost, 48.0) { // 6000 DPM * 0.008
		t.Errorf("metrics cost: want 48.0, got %v", metrics.Cost)
	}
	if !approx(got.CurrentCost, 1.0+0.5+48.0) {
		t.Errorf("current total: want 49.5, got %v", got.CurrentCost)
	}
	if got.DaysElapsed != 15 || got.DaysInMonth != 30 {
		t.Errorf("billing period: want day 15/30, got %d/%d", got.DaysElapsed, got.DaysInMonth)
	}
}

func TestEstimateCostZeroWindowAndDaysAreSafe(t *testing.T) {
	got := estimateCost(usageQuantities{metricDPs: 100}, testRates)
	if got.CurrentCost != 0 {
		t.Errorf("zero window/days must not divide by zero: %+v", got)
	}
}
