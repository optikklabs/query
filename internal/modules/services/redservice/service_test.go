package redservice

import (
	"math"
	"testing"
)

// normalizeUtilization scales fractions (<=1) to percent, keeps 1..100 as-is,
// and drops invalid or out-of-range values.
func TestNormalizeUtilization(t *testing.T) {
	cases := []struct {
		in      float64
		wantNil bool
		want    float64
	}{
		{0.5, false, 50},
		{1.0, false, 100},
		{50, false, 50},
		{100, false, 100},
		{100.1, true, 0},
		{-1, true, 0},
		{math.NaN(), true, 0},
		{math.Inf(1), true, 0},
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
		t.Errorf("empty input must return nil, got %v", *got)
	}
	if got := averageFloats([]float64{10, 20, 30}); got == nil || *got != 20 {
		t.Errorf("averageFloats([10,20,30]) = %v, want 20", got)
	}
}
