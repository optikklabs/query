package ingestion

import (
	"math"
	"testing"
)

func TestEstimateCostPricesMetricsPerMillionSamples(t *testing.T) {
	got := estimateCost(usageQuantities{
		logsBytes:     2_000_000_000,
		spansBytes:    3_000_000_000,
		metricSamples: 2_500_000,
		daysElapsed:   15,
		daysInMonth:   31,
	}, Rates{
		Currency:                "USD",
		PerGBLogsTraces:         0.10,
		PerMillionMetricSamples: 0.10,
	})

	if len(got.Lines) != 3 {
		t.Fatalf("got %d cost lines, want 3", len(got.Lines))
	}
	metrics := got.Lines[2]
	if metrics.Unit != "million samples" {
		t.Errorf("metrics unit = %q, want %q", metrics.Unit, "million samples")
	}
	if metrics.Quantity != 2.5 {
		t.Errorf("metrics quantity = %v, want 2.5", metrics.Quantity)
	}
	if metrics.Rate != 0.10 {
		t.Errorf("metrics rate = %v, want 0.10", metrics.Rate)
	}
	if metrics.Cost != 0.25 {
		t.Errorf("metrics cost = %v, want 0.25", metrics.Cost)
	}
	if math.Abs(got.CurrentCost-0.75) > 1e-9 {
		t.Errorf("current cost = %v, want 0.75", got.CurrentCost)
	}
}
