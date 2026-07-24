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
		{int64(time.Hour / time.Millisecond), "toStartOfInterval(timestamp, INTERVAL 60 SECOND)"},
		{12 * int64(time.Hour/time.Millisecond), "toStartOfInterval(timestamp, INTERVAL 300 SECOND)"},
		{3 * 24 * int64(time.Hour/time.Millisecond), "toStartOfInterval(timestamp, INTERVAL 3600 SECOND)"},
		{30 * 24 * int64(time.Hour/time.Millisecond), "toStartOfInterval(timestamp, INTERVAL 86400 SECOND)"},
	}
	for _, c := range cases {
		if got := DisplayGrainSQL(c.windowMs); got != c.want {
			t.Errorf("DisplayGrainSQL(%dms) = %q, want %q", c.windowMs, got, c.want)
		}
	}
}
