package errors

import (
	"testing"

	"github.com/optikklabs/query/internal/shared/metrics"
)

func TestHTTPBucketToCode(t *testing.T) {
	cases := map[string]int{"4xx": 400, "5xx": 500, "": 0, "2xx": 0}
	for in, want := range cases {
		if got := httpBucketToCode(in); got != want {
			t.Errorf("httpBucketToCode(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestComputeErrorRate(t *testing.T) {
	cases := []struct {
		errs, total int64
		want        float64
	}{
		{0, 0, 0}, // div-by-zero guard
		{5, 0, 0},
		{1, 4, 25},
		{3, 3, 100},
	}
	for _, c := range cases {
		if got := metrics.ComputeErrorRate(c.errs, c.total); got != c.want {
			t.Errorf("metrics.ComputeErrorRate(%d,%d) = %v, want %v", c.errs, c.total, got, c.want)
		}
	}
}

func TestComputeAvgLatency(t *testing.T) {
	if got := metrics.ComputeAvgLatency(100, 0); got != 0 {
		t.Errorf("zero count must avoid div-by-zero, got %v", got)
	}
	if got := metrics.ComputeAvgLatency(100, 4); got != 25 {
		t.Errorf("metrics.ComputeAvgLatency(100,4) = %v, want 25", got)
	}
}

func TestFacetPct(t *testing.T) {
	if got := metrics.FacetPercentage(1, 0); got != 0 {
		t.Errorf("zero total must return 0, got %v", got)
	}
	if got := metrics.FacetPercentage(1, 4); got != 25 {
		t.Errorf("metrics.FacetPercentage(1,4) = %v, want 25", got)
	}
}
