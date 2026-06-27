package timebucket

import "testing"

// FloorMsToBucket must snap a mid-bucket start down to the bucket boundary so the
// leading partial bucket's rollup row (stored at the boundary) is included.
func TestFloorMsToBucket(t *testing.T) {
	cases := []struct {
		ms, bucketSec, want int64
	}{
		{90_000, 60, 60_000},
		{60_000, 60, 60_000},
		{370_000, 300, 300_000},
		{3_700_000, 3600, 3_600_000},
		{0, 60, 0},
	}
	for _, c := range cases {
		if got := FloorMsToBucket(c.ms, c.bucketSec); got != c.want {
			t.Errorf("FloorMsToBucket(%d,%d)=%d want %d", c.ms, c.bucketSec, got, c.want)
		}
	}
}
