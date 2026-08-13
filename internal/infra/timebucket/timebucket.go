package timebucket

import (
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func DisplayGrain(windowMs int64) time.Duration {
	return time.Duration(displayGrainSeconds(windowMs)) * time.Second
}

const maxBucketPoints int64 = 300

var displayGrainLadder = []int64{60, 300, 3600, 86400}

func displayGrainSeconds(windowMs int64) int64 {
	return grainSecondsFor(displayGrainLadder, windowMs/1000)
}

func grainSecondsFor(ladder []int64, windowSeconds int64) int64 {
	for _, g := range ladder {
		if windowSeconds/g <= maxBucketPoints {
			return g
		}
	}
	return ladder[len(ladder)-1]
}

func DisplayGrainSQL(windowMs int64) string {
	return GrainSQL(displayGrainSeconds(windowMs))
}

func RollupTableForGrain(grainSec int64) string {
	switch rollupGrainSeconds(grainSec) {
	case 60:
		return "optikk.metrics_1m_v2"
	case 300:
		return "optikk.metrics_5m_v2"
	default:
		return "optikk.metrics_1h_v2"
	}
}

func GrainSQL(grainSec int64) string {
	if grainSec <= 0 {
		grainSec = 300
	}
	return fmt.Sprintf("toStartOfInterval(timestamp, INTERVAL %d SECOND)", grainSec)
}

func FloorMsToBucket(ms, bucketSec int64) int64 {
	return ms - ms%(bucketSec*1000)
}

func MetricsRollup(startMs, endMs int64) string {
	return RollupTableForGrain(displayGrainSeconds(endMs - startMs))
}

func RollupGrainSeconds(windowMs int64) int64 {
	return rollupGrainSeconds(displayGrainSeconds(windowMs))
}

func rollupGrainSeconds(grainSec int64) int64 {
	switch {
	case grainSec < 300:
		return 60
	case grainSec < 3600:
		return 300
	default:
		return 3600
	}
}

func SpanStatsRollup(startMs, endMs int64) string {
	switch RollupGrainSeconds(endMs - startMs) {
	case 60:
		return "optikk.span_stats_1m"
	case 300:
		return "optikk.span_stats_5m"
	default:
		return "optikk.span_stats_1h"
	}
}

func WithBucketGrainSec(args []any, startMs, endMs int64) []any {
	return append(args, clickhouse.Named("bucketGrainSec", displayGrainSeconds(endMs-startMs)))
}

func DenseBuckets(startMs, endMs int64, grain time.Duration) []time.Time {
	start := time.UnixMilli(startMs).UTC().Truncate(grain)
	var out []time.Time
	for b := start; b.Before(time.UnixMilli(endMs)); b = b.Add(grain) {
		out = append(out, b)
	}
	return out
}

func BuildDenseTimestamps(startMs, endMs int64, bucketSec int64) []int64 {
	flooredStart := FloorMsToBucket(startMs, bucketSec)
	bucketMs := bucketSec * 1000

	var ts []int64
	for t := flooredStart; t < endMs; t += bucketMs {
		ts = append(ts, t)
	}
	return ts
}

// FillGaps builds one point per grain bucket in [startMs, endMs).
// Rows are keyed by their truncated bucket via at; point receives the
// zero row and ok=false for buckets that have no matching row.
func FillGaps[R, P any](
	startMs, endMs int64, grain time.Duration, rows []R,
	at func(R) time.Time,
	point func(t time.Time, row R, ok bool) P,
) []P {
	byBucket := make(map[int64]R, len(rows))
	for _, r := range rows {
		byBucket[at(r).UTC().Truncate(grain).Unix()] = r
	}
	buckets := DenseBuckets(startMs, endMs, grain)
	points := make([]P, 0, len(buckets))
	for _, t := range buckets {
		row, ok := byBucket[t.Unix()]
		points = append(points, point(t, row, ok))
	}
	return points
}

// FillGapsKeyed groups rows by key and builds one dense series per key over
// the grain buckets in [startMs, endMs). Keys keep first-seen order; point
// receives the zero row and ok=false for buckets with no matching row.
func FillGapsKeyed[R, P any](
	startMs, endMs int64, grain time.Duration, rows []R,
	key func(R) string,
	at func(R) time.Time,
	point func(t time.Time, row R, ok bool) P,
) (keys []string, buckets []time.Time, series [][]P) {
	byKey := make(map[string]map[int64]R)
	for _, r := range rows {
		k := key(r)
		if _, ok := byKey[k]; !ok {
			byKey[k] = make(map[int64]R)
			keys = append(keys, k)
		}
		byKey[k][at(r).UTC().Truncate(grain).Unix()] = r
	}
	buckets = DenseBuckets(startMs, endMs, grain)
	series = make([][]P, len(keys))
	for i, k := range keys {
		points := make([]P, len(buckets))
		for j, t := range buckets {
			row, ok := byKey[k][t.Unix()]
			points[j] = point(t, row, ok)
		}
		series[i] = points
	}
	return keys, buckets, series
}

func ZeroFillGaps(values []*float64) {
	zero := 0.0
	for i, v := range values {
		if v == nil {
			values[i] = &zero
		}
	}
}
