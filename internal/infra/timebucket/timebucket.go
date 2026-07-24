// Package timebucket provides the source of truth for system bucket values.
// Both writers and readers call the same helpers to align bucket values.
package timebucket

import (
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
		return "optikk.metrics_1m"
	case grainSec < 3600:
		return "optikk.metrics_5m"
	default:
		return "optikk.metrics_1h"
	}
}

func GrainSQL(grainSec int64) string {
	switch grainSec {
	case 60:
		return "toStartOfMinute(timestamp)"
	case 300:
		return "toStartOfFiveMinutes(timestamp)"
	case 900:
		return "toStartOfFifteenMinutes(timestamp)"
	case 3600:
		return "toStartOfHour(timestamp)"
	case 86400:
		return "toStartOfDay(timestamp)"
	default:
		return "toStartOfFiveMinutes(timestamp)"
	}
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

func MetricsHistRollup(windowMs int64) string {
	return MetricsRollup(windowMs)
}

func WithBucketGrainSec(args []any, startMs, endMs int64) []any {
	sec := int64(displayGrain(endMs - startMs).Seconds())
	if sec <= 0 {
		sec = 60
	}
	return append(args, clickhouse.Named("bucketGrainSec", sec))
}

// DenseBuckets returns an ordered slice of time.Time buckets aligned to grain.
func DenseBuckets(startMs, endMs int64, grain time.Duration) []time.Time {
	start := time.UnixMilli(startMs).UTC().Truncate(grain)
	end := time.UnixMilli(endMs).UTC().Truncate(grain)
	var out []time.Time
	for b := start; !b.After(end); b = b.Add(grain) {
		out = append(out, b)
	}
	return out
}

// BuildDenseTimestamps generates an ordered slice of bucket-aligned millisecond timestamps.
func BuildDenseTimestamps(startMs, endMs int64, bucketSec int64) []int64 {
	flooredStart := FloorMsToBucket(startMs, bucketSec)
	bucketMs := bucketSec * 1000

	var ts []int64
	for t := flooredStart; t <= endMs; t += bucketMs {
		ts = append(ts, t)
	}
	return ts
}

// ZeroFillGaps replaces nil entries in values with pointers to 0.0.
func ZeroFillGaps(values []*float64) {
	zero := 0.0
	for i, v := range values {
		if v == nil {
			values[i] = &zero
		}
	}
}
