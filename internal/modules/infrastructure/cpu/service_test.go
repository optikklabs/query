package cpu

import (
	"math"
	"testing"

	"github.com/optikklabs/query/internal/modules/infrastructure/infraconsts"
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
}

// foldCPUMetricRows normalizes each present CPU metric and averages them; a
// fraction and a percent for the same logical value reconcile to one number.
func TestFoldCPUMetricRows(t *testing.T) {
	rows := []CPUMetricNameRow{
		{MetricName: infraconsts.MetricSystemCPUUtilization, Value: 0.40},
		{MetricName: infraconsts.MetricProcessCPUUsage, Value: 60},
	}
	got := foldCPUMetricRows(rows)
	if got == nil || *got != 50 {
		t.Errorf("fold = %v, want 50 (avg of 40,60)", got)
	}
}

func TestFoldCPUMetricRows_NoneValid(t *testing.T) {
	rows := []CPUMetricNameRow{{MetricName: infraconsts.MetricSystemCPUUsage, Value: 999}}
	if got := foldCPUMetricRows(rows); got != nil {
		t.Errorf("out-of-range only -> nil, got %v", *got)
	}
}
