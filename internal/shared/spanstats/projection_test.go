package spanstats

import (
	"strings"
	"testing"
)

// The whole point of the package is that a measure's alias never collides with
// a column it reads: ClickHouse resolves aliases ahead of columns, so a
// colliding alias makes a sibling aggregate fail with ILLEGAL_AGGREGATION.
func TestMeasureAliasesNeverShadowTheirSourceColumns(t *testing.T) {
	for _, tc := range []struct {
		name  string
		proj  string
		alias string
	}{
		{"Requests", Requests, RequestTotal},
		{"Errors", Errors, ErrorTotal},
		{"DurationSum", DurationSum, DurationTotal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expr, alias, ok := strings.Cut(tc.proj, " AS ")
			if !ok {
				t.Fatalf("projection %q has no AS clause", tc.proj)
			}
			if alias != tc.alias {
				t.Errorf("alias = %q, want %q", alias, tc.alias)
			}
			if strings.Contains(expr, alias) {
				t.Errorf("alias %q also appears in its own expression %q: "+
					"ClickHouse would resolve later references to the aggregate", alias, expr)
			}
		})
	}
}

func TestErrorsProjectionCarriesTheErrorPredicate(t *testing.T) {
	if !strings.Contains(Errors, ErrorPred) {
		t.Errorf("Errors = %q, want it to filter on %q", Errors, ErrorPred)
	}
}

func TestLatencySQL(t *testing.T) {
	for _, tc := range []struct {
		name string
		l    Latency
		want string
	}{
		{"three quantiles", LatencyP50P95P99, "quantilesTDigestMerge(0.5, 0.95, 0.99)(latency_state) AS qs"},
		{"two quantiles", LatencyP50P95, "quantilesTDigestMerge(0.5, 0.95)(latency_state) AS qs"},
		{"p95 only", LatencyP95, "quantilesTDigestMerge(0.95)(latency_state) AS qs"},
		{"p99 only", LatencyP99, "quantilesTDigestMerge(0.99)(latency_state) AS qs"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.l.SQL(); got != tc.want {
				t.Errorf("SQL() = %q, want %q", got, tc.want)
			}
		})
	}
}

// At must resolve the index from the projection that produced the column, so
// the same qs position means different quantiles for different arities.
func TestAtResolvesIndexFromTheProjection(t *testing.T) {
	three := []float64{10, 20, 30}
	if got := LatencyP50P95P99.At(three, P99); got != 30 {
		t.Errorf("p99 of three-quantile row = %v, want 30", got)
	}
	if got := LatencyP50P95.At([]float64{10, 20}, P95); got != 20 {
		t.Errorf("p95 of two-quantile row = %v, want 20", got)
	}
	// p95 sits at index 0 here, not index 1, because that is all this
	// projection selected.
	if got := LatencyP95.At([]float64{99}, P95); got != 99 {
		t.Errorf("p95 of p95-only row = %v, want 99", got)
	}
}

func TestAtPanicsOnUnprojectedQuantile(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("reading p99 from a p50/p95 projection should panic, not return zero")
		}
	}()
	LatencyP50P95.At([]float64{10, 20}, P99)
}

func TestDecodeHelpers(t *testing.T) {
	p50, p95, p99 := LatencyP50P95P99.P50P95P99([]float64{1, 2, 3})
	if p50 != 1 || p95 != 2 || p99 != 3 {
		t.Errorf("P50P95P99 = %v/%v/%v, want 1/2/3", p50, p95, p99)
	}
	e50, e95 := LatencyP50P95.P50P95([]float64{4, 5})
	if e50 != 4 || e95 != 5 {
		t.Errorf("P50P95 = %v/%v, want 4/5", e50, e95)
	}
}
