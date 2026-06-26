// Package timebucket provides the source of truth for system bucket values.
// Both writers and readers call the same helpers to align bucket values.
package timebucket

import (
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

const BucketSeconds int64 = 300

func BucketStart(unixSeconds int64) uint32 {
	return uint32((unixSeconds / BucketSeconds) * BucketSeconds)
}

func FormatDisplayBucket(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}

func DisplayBucket(rowUnixSeconds int64, windowMs int64) time.Time {
	return bucketAt(rowUnixSeconds, displayGrain(windowMs))
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

func bucketAt(unixSeconds int64, grain time.Duration) time.Time {
	return time.Unix(unixSeconds, 0).UTC().Truncate(grain)
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
