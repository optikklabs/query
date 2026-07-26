package infraconsts

import (
	"math"
	"testing"
)

// Cases merged from the two modules that each carried their own copy of this
// function: infrastructure and services/redfleet.
func TestNormalizeUtilization(t *testing.T) {
	cases := []struct {
		in      float64
		wantNil bool
		want    float64
	}{
		{0.5, false, 50},
		{1.0, false, 100},
		{50, false, 50},
		{75, false, 75},
		{100, false, 100},
		{100.1, true, 0},
		{-0.1, true, 0},
		{-1, true, 0},
		{math.NaN(), true, 0},
		{math.Inf(1), true, 0},
	}
	for _, c := range cases {
		got := NormalizeUtilization(c.in)
		if c.wantNil {
			if got != nil {
				t.Errorf("NormalizeUtilization(%v) = %v, want nil", c.in, *got)
			}
			continue
		}
		if got == nil || *got != c.want {
			t.Errorf("NormalizeUtilization(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestAverageUtilization(t *testing.T) {
	if got := AverageUtilization(nil); got != nil {
		t.Errorf("empty input must return nil, got %v", *got)
	}
	if got := AverageUtilization([]float64{10, 30}); got == nil || *got != 20 {
		t.Errorf("AverageUtilization([10,30]) = %v, want 20", got)
	}
	if got := AverageUtilization([]float64{10, 20, 30}); got == nil || *got != 20 {
		t.Errorf("AverageUtilization([10,20,30]) = %v, want 20", got)
	}
}
