// Package timebucket provides the source of truth for system bucket values.
// Both writers and readers call the same helpers to align bucket values.
package timebucket

import (
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

const BucketSeconds int64 = 300

// BucketStart truncates a Unix-second timestamp to its 5-minute bucket.
// Returned as UInt32 for lossless round-trips through ClickHouse.
func BucketStart(unixSeconds int64) uint32 {
	return uint32((unixSeconds / BucketSeconds) * BucketSeconds)
}

// FormatDisplayBucket formats a display-grain bucket time.Time as UTC
// "YYYY-MM-DD HH:MM:SS" for consistency with ClickHouse string formats.
func FormatDisplayBucket(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}

// DisplayBucket buckets a row timestamp into a display-time bucket.
// Grain is based on query window: <=24h: 5m, <=7d: 1h, else 1d. 1m is the
// finest tier (raw -> 1m -> 1h), so there is no sub-1m display grain.
func DisplayBucket(rowUnixSeconds int64, windowMs int64) time.Time {
	return bucketAt(rowUnixSeconds, displayGrain(windowMs))
}

func DisplayGrain(windowMs int64) time.Duration {
	return displayGrain(windowMs)
}

// MaxBucketPoints caps points-per-graph, mirroring Datadog's ~300/graph rule.
const MaxBucketPoints int64 = 300

// displayGrainLadder are the table-aligned display grains (1m, 5m, 1h, 1d). The
// readers pick their rollup table via UseHourRollup, so the grain must never be
// finer than the chosen table's native resolution.
var displayGrainLadder = []int64{60, 300, 3600, 86400}

// GrainSecondsFor picks the finest grain in the ladder keeping the point count
// (windowSeconds/grain) at or under MaxBucketPoints.
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

// DisplayGrainSQL returns the fastest CH SQL fragment for adaptive grain.
// The returned fragment expects a `timestamp` column in scope.
func DisplayGrainSQL(windowMs int64) string {
	switch displayGrain(windowMs) {
	case time.Minute:
		return "toStartOfMinute(timestamp)"
	case 5 * time.Minute:
		return "toStartOfFiveMinutes(timestamp)"
	case time.Hour:
		return "toStartOfHour(timestamp)"
	default:
		return "toStartOfDay(timestamp)"
	}
}

// UseHourRollup reports whether a query window should read the 1h rollup
// tier instead of the 5m tier. True exactly when the display grain is >= 1h
// (window > 24h), so hourly rows always satisfy the display grain losslessly.
func UseHourRollup(windowMs int64) bool {
	return displayGrain(windowMs) >= time.Hour
}

// FloorMsToHour floors a Unix-ms timestamp to its hour boundary. Used to snap
// the window start when routing to the 1h tier: hourly rows are all-or-nothing,
// and the first display bucket is hour-labeled either way.
func FloorMsToHour(ms int64) int64 {
	return ms - ms%(3600*1000)
}

// SnapRangeForRollup floors the window start to its hour when the window
// routes to the 1h tier. Use at the top of rollup readers that build their
// ClickHouse args manually (the chargs.RollupRangeArgs path does this itself).
func SnapRangeForRollup(startMs, endMs int64) (int64, int64) {
	if UseHourRollup(endMs - startMs) {
		return FloorMsToHour(startMs), endMs
	}
	return startMs, endMs
}

// MetricsRollup returns the scalar rollup table for the window: 1m for windows
// up to the 1h-routing threshold, 1h beyond it.
func MetricsRollup(windowMs int64) string {
	if UseHourRollup(windowMs) {
		return "optikk.metrics_1h"
	}
	return "optikk.metrics_1m"
}

// MetricsHistRollup returns the rollup table for histogram queries.
// Now unified with scalar rollups — both carry histogram state.
func MetricsHistRollup(windowMs int64) string {
	return MetricsRollup(windowMs)
}

// WithBucketGrainSec appends @bucketGrainSec in seconds matching DisplayGrain.
// Used to compute rates from counts within each display bucket.
func WithBucketGrainSec(args []any, startMs, endMs int64) []any {
	sec := int64(displayGrain(endMs - startMs).Seconds())
	if sec <= 0 {
		sec = 60
	}
	return append(args, clickhouse.Named("bucketGrainSec", sec))
}
