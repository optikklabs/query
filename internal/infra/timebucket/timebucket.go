package timebucket

import (
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

const BucketSeconds int64 = 300

func FormatDisplayBucket(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}

func DisplayGrain(windowMs int64) time.Duration {
	return displayGrain(windowMs)
}

const MaxBucketPoints int64 = 300

var displayGrainLadder = []int64{60, 300, 3600, 86400}

func GrainSecondsFor(ladder []int64, windowSeconds int64) int64 {
	for _, g := range ladder {
		if windowSeconds/g <= MaxBucketPoints {
			return g
		}
	}
	return ladder[len(ladder)-1]
}

func displayGrain(windowMs int64) time.Duration {
	sec := GrainSecondsFor(displayGrainLadder, windowMs/1000)
	return time.Duration(sec) * time.Second
}

func DisplayGrainSQL(windowMs int64) string {
	return GrainSQL(int64(displayGrain(windowMs).Seconds()))
}

func RollupTableForGrain(grainSec int64) string {
	switch {
	case grainSec < 300:
		return "optikk.metrics_1m_v2"
	case grainSec < 3600:
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

func UseHourRollup(windowMs int64) bool {
	return displayGrain(windowMs) >= time.Hour
}

func FloorMsToHour(ms int64) int64 {
	return ms - ms%(3600*1000)
}

func FloorMsToBucket(ms, bucketSec int64) int64 {
	return ms - ms%(bucketSec*1000)
}

func SnapRangeForRollup(startMs, endMs int64) (int64, int64) {
	if UseHourRollup(endMs - startMs) {
		return FloorMsToHour(startMs), endMs
	}
	return startMs, endMs
}

func MetricsRollup(windowMs int64) string {
	return RollupTableForGrain(int64(displayGrain(windowMs).Seconds()))
}

func SpanStatsRollup(windowMs int64) string {
	switch grainSec := int64(displayGrain(windowMs).Seconds()); {
	case grainSec < 300:
		return "optikk.span_stats_1m"
	case grainSec < 3600:
		return "optikk.span_stats_5m"
	default:
		return "optikk.span_stats_1h"
	}
}

func WithBucketGrainSec(args []any, startMs, endMs int64) []any {
	sec := int64(displayGrain(endMs - startMs).Seconds())
	if sec <= 0 {
		sec = 60
	}
	return append(args, clickhouse.Named("bucketGrainSec", sec))
}

func DenseBuckets(startMs, endMs int64, grain time.Duration) []time.Time {
	start := time.UnixMilli(startMs).UTC().Truncate(grain)
	end := time.UnixMilli(endMs).UTC().Truncate(grain)
	var out []time.Time
	for b := start; !b.After(end); b = b.Add(grain) {
		out = append(out, b)
	}
	return out
}

func BuildDenseTimestamps(startMs, endMs int64, bucketSec int64) []int64 {
	flooredStart := FloorMsToBucket(startMs, bucketSec)
	bucketMs := bucketSec * 1000

	var ts []int64
	for t := flooredStart; t <= endMs; t += bucketMs {
		ts = append(ts, t)
	}
	return ts
}

// FillGaps builds one point per grain bucket in [startMs, endMs].
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
// the grain buckets in [startMs, endMs]. Keys keep first-seen order; point
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
