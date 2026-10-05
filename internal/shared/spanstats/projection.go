package spanstats

import (
	"strconv"
	"strings"
)

const (
	Requests = "sum(request_count) AS " + RequestTotal

	Errors = "sumIf(request_count, " + ErrorPred + ") AS " + ErrorTotal

	DurationSum = "sum(duration_ms_sum) AS " + DurationTotal
)

const (
	RequestTotal  = "request_total"
	ErrorTotal    = "error_total"
	DurationTotal = "duration_ms_total"

	LatencyAlias = "qs"
)

type Latency struct {
	quantiles []float64
}

var (
	LatencyP50P95P99 = Latency{quantiles: []float64{0.5, 0.95, 0.99}}
	LatencyP50P95    = Latency{quantiles: []float64{0.5, 0.95}}
	LatencyP95       = Latency{quantiles: []float64{0.95}}
	LatencyP99       = Latency{quantiles: []float64{0.99}}
)

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

func (l Latency) At(qs []float64, q float64) float64 {
	for i, have := range l.quantiles {
		if have == q {
			return qs[i]
		}
	}
	panic("spanstats: quantile " + strconv.FormatFloat(q, 'g', -1, 64) +
		" not projected by this Latency")
}

const (
	P50 = 0.5
	P95 = 0.95
	P99 = 0.99
)

func (l Latency) P50P95P99(qs []float64) (p50, p95, p99 float64) {
	return l.At(qs, P50), l.At(qs, P95), l.At(qs, P99)
}

func (l Latency) P50P95(qs []float64) (p50, p95 float64) {
	return l.At(qs, P50), l.At(qs, P95)
}
