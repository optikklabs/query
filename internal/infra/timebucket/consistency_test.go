package timebucket

import (
	"testing"
	"time"
)

// BucketSeconds is baked into the spans/logs/metrics PKs and both rollup MVs.
// Changing it is a breaking schema change requiring a table rebuild.
func TestBucketSecondsInvariant(t *testing.T) {
	if BucketSeconds != 300 {
		t.Fatalf("BucketSeconds = %d; changing it requires a CH table rebuild", BucketSeconds)
	}
}

func TestBucketStartMatchesMVDerivation(t *testing.T) {
	cases := []int64{
		0, 1, 299, 300, 301, 599, 600,
		1735689600,
		1735689600 + 7,
		1735689600 + 299,
	}
	for _, s := range cases {
		want := uint32((s / 300) * 300)
		if got := BucketStart(s); got != want {
			t.Errorf("BucketStart(%d) = %d, want %d", s, got, want)
		}
	}
}

func TestDisplayGrainWindows(t *testing.T) {
	hourMs := int64(time.Hour / time.Millisecond)
	cases := []struct {
		windowMs int64
		want     time.Duration
	}{
		{hourMs, time.Minute},
		{3 * hourMs, time.Minute},
		{5 * hourMs, time.Minute},
		{6 * hourMs, 5 * time.Minute},
		{24 * hourMs, 5 * time.Minute},
		{25 * hourMs, 5 * time.Minute},
		{26 * hourMs, time.Hour},
		{7 * 24 * hourMs, time.Hour},
		{12 * 24 * hourMs, time.Hour},
		{13 * 24 * hourMs, 24 * time.Hour},
		{30 * 24 * hourMs, 24 * time.Hour},
	}
	for _, c := range cases {
		if got := DisplayGrain(c.windowMs); got != c.want {
			t.Errorf("DisplayGrain(%dms) = %v, want %v", c.windowMs, got, c.want)
		}
	}
}

func TestGrainSecondsForCapsPoints(t *testing.T) {
	ladder := []int64{60, 300, 3600, 86400}
	hour := int64(3600)
	for _, windowSec := range []int64{hour, 24 * hour, 7 * 24 * hour, 30 * 24 * hour, 90 * 24 * hour} {
		g := GrainSecondsFor(ladder, windowSec)
		if windowSec/g > MaxBucketPoints {
			t.Errorf("window %ds at grain %ds = %d points, want <= %d", windowSec, g, windowSec/g, MaxBucketPoints)
		}
	}
}

func TestDisplayGrainSQLDispatch(t *testing.T) {
	cases := []struct {
		windowMs int64
		want     string
	}{
		{int64(time.Hour / time.Millisecond), "toStartOfMinute(timestamp)"},
		{12 * int64(time.Hour/time.Millisecond), "toStartOfFiveMinutes(timestamp)"},
		{3 * 24 * int64(time.Hour/time.Millisecond), "toStartOfHour(timestamp)"},
		{30 * 24 * int64(time.Hour/time.Millisecond), "toStartOfDay(timestamp)"},
	}
	for _, c := range cases {
		if got := DisplayGrainSQL(c.windowMs); got != c.want {
			t.Errorf("DisplayGrainSQL(%dms) = %q, want %q", c.windowMs, got, c.want)
		}
	}
}

func TestDisplayBucketTruncation(t *testing.T) {
	rowSec := int64(1735693271)
	cases := []struct {
		windowMs int64
		want     string
	}{
		{int64(time.Hour / time.Millisecond), "2025-01-01 01:01:00"},
		{12 * int64(time.Hour/time.Millisecond), "2025-01-01 01:00:00"},
		{3 * 24 * int64(time.Hour/time.Millisecond), "2025-01-01 01:00:00"},
		{30 * 24 * int64(time.Hour/time.Millisecond), "2025-01-01 00:00:00"},
	}
	for _, c := range cases {
		if got := FormatDisplayBucket(DisplayBucket(rowSec, c.windowMs)); got != c.want {
			t.Errorf("DisplayBucket(%d, %dms) = %q, want %q", rowSec, c.windowMs, got, c.want)
		}
	}
}
