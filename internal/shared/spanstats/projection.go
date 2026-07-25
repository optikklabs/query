package spanstats

import (
	"strconv"
	"strings"
)

// Measure projections for the span_stats rollup.
//
// Each constant pairs an aggregate expression with its alias. Keeping the
// pairing in exactly one place is what makes alias shadowing impossible: the
// ClickHouse analyzer resolves an alias ahead of the physical column, so
// `sum(request_count) AS request_count` makes every later reference to
// request_count resolve to the aggregate instead of the column, which fails
// with ILLEGAL_AGGREGATION (code 184). The aliases below deliberately differ
// from the columns they read.
const (
	// Requests is the total request count over the selected rows.
	Requests = "sum(request_count) AS " + RequestTotal

	// Errors is the subset of Requests whose span status is ERROR.
	Errors = "sumIf(request_count, " + ErrorPred + ") AS " + ErrorTotal

	// DurationSum is the total wall time, used to derive average latency.
	DurationSum = "sum(duration_ms_sum) AS " + DurationTotal
)

// Aliases produced by the measure projections above. Repositories reference
// these in GROUP BY, HAVING, and ORDER BY clauses, and row structs bind to
// them via `ch:` tags, so the names live here rather than being retyped.
const (
	RequestTotal  = "request_total"
	ErrorTotal    = "error_total"
	DurationTotal = "duration_ms_total"

	// LatencyAlias is the column produced by Latency.SQL.
	LatencyAlias = "qs"
)

// Latency pairs a tDigest projection with the decoder for its result, so the
// projected quantile arity and the decoded arity cannot drift apart. Use one
// of the package-level values; the zero value projects nothing.
type Latency struct {
	quantiles []float64
}

// The quantile sets the read paths actually need. Not every consumer wants all
// three — topology edges render p50/p95 only — and projecting unused quantiles
// costs merge work per row, so the arities stay distinct.
var (
	LatencyP50P95P99 = Latency{quantiles: []float64{0.5, 0.95, 0.99}}
	LatencyP50P95    = Latency{quantiles: []float64{0.5, 0.95}}
	LatencyP95       = Latency{quantiles: []float64{0.95}}
	LatencyP99       = Latency{quantiles: []float64{0.99}}
)

// SQL renders the projection, e.g.
// `quantilesTDigestMerge(0.5, 0.95)(latency_state) AS qs`.
func (l Latency) SQL() string {
	var b strings.Builder
	b.WriteString("quantilesTDigestMerge(")
	for i, q := range l.quantiles {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.FormatFloat(q, 'g', -1, 64))
	}
	b.WriteString(")(latency_state) AS ")
	b.WriteString(LatencyAlias)
	return b.String()
}

// At returns the value for quantile q from a row's decoded qs column.
//
// quantilesTDigestMerge always returns exactly as many values as were
// projected — an empty state yields [nan, nan, nan] rather than a short array —
// so indexing needs no length guard. The index comes from the same quantile
// list that built the projection, which is what stops a caller reading p99 out
// of a projection that never selected it.
//
// Asking for an unprojected quantile is a programming error, not a data
// condition: it cannot be triggered by input and fails on first execution.
// Panicking keeps that honest, where returning zero would render as a real 0ms
// latency in the UI.
func (l Latency) At(qs []float64, q float64) float64 {
	for i, have := range l.quantiles {
		if have == q {
			return qs[i]
		}
	}
	panic("spanstats: quantile " + strconv.FormatFloat(q, 'g', -1, 64) +
		" not projected by this Latency")
}

// The quantiles the read paths request, named so call sites read as intent
// rather than as magic numbers.
const (
	P50 = 0.5
	P95 = 0.95
	P99 = 0.99
)

// P50P95P99 decodes the three-quantile projection. Values are milliseconds, in
// ClickHouse's native float64; callers narrow to float32 where their row type
// does.
func (l Latency) P50P95P99(qs []float64) (p50, p95, p99 float64) {
	return l.At(qs, P50), l.At(qs, P95), l.At(qs, P99)
}

// P50P95 decodes the two-quantile projection used by service-graph edges.
func (l Latency) P50P95(qs []float64) (p50, p95 float64) {
	return l.At(qs, P50), l.At(qs, P95)
}
