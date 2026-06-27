package memory

import (
	"testing"

	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
)

func TestAverageFloats(t *testing.T) {
	if got := averageFloats(nil); got != nil {
		t.Errorf("empty must be nil, got %v", *got)
	}
	if got := averageFloats([]float64{10, 30}); got == nil || *got != 20 {
		t.Errorf("averageFloats([10,30]) = %v, want 20", got)
	}
}

// system memory utilization fraction scales to percent; JVM used/max derives a
// percent; the two average together.
func TestFoldMemoryMetricRows(t *testing.T) {
	rows := []MemoryMetricNameRow{
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
	rows := []MemoryMetricNameRow{
		{MetricName: infraconsts.MetricJVMMemoryUsed, Value: 120},
		{MetricName: infraconsts.MetricJVMMemoryMax, Value: 0},
	}
	if got := foldMemoryMetricRows(rows); got != nil {
		t.Errorf("zero max -> nil, got %v", *got)
	}
}
