package mathutil

import (
	"math"
	"sort"
)

// HistogramTuple matches the ClickHouse sumMap output shape.
type HistogramTuple []any

func Quantiles(q []float64, hist HistogramTuple) []float32 {
	out := make([]float32, len(q))
	if len(hist) != 2 {
		return out
	}
	buckets, okB := hist[0].([]float64)
	counts, okC := hist[1].([]uint64)
	if !okB || !okC || len(buckets) == 0 || len(buckets) != len(counts) {

		return out
	}

	type bc struct {
		b float64
		c uint64
	}
	pairs := make([]bc, len(buckets))
	for i := range buckets {
		pairs[i] = bc{b: buckets[i], c: counts[i]}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].b < pairs[j].b })

	var total uint64
	for _, p := range pairs {
		total += p.c
	}
	if total == 0 {
		return out
	}

	for i, quantile := range q {
		target := float64(total) * quantile
		var sum uint64
		for j, p := range pairs {
			sum += p.c
			if float64(sum) >= target {
				if p.c == 0 {
					out[i] = float32(p.b)
					break
				}

				prevSum := sum - p.c
				prevB := 0.0
				if j > 0 {
					prevB = pairs[j-1].b
				}

				if math.IsInf(p.b, 1) {
					out[i] = float32(prevB)
					break
				}

				frac := (target - float64(prevSum)) / float64(p.c)
				val := prevB + frac*(p.b-prevB)
				out[i] = float32(val)
				break
			}
		}
	}
	return out
}
