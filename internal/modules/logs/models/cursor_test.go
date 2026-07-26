package models

import (
	"testing"
	"time"
)

// The log cursor carries log_id as the unique tiebreaker and preserves the
// nanosecond timestamp through encode/decode.
func TestLogCursorRoundTrip(t *testing.T) {
	in := Cursor{Timestamp: time.Unix(0, 1_700_000_000_123_456_789).UTC(), LogID: "log-xyz"}
	out, ok := DecodeCursor(in.Encode())
	if !ok {
		t.Fatal("decode failed")
	}
	if !out.Timestamp.Equal(in.Timestamp) {
		t.Errorf("Timestamp=%v want %v", out.Timestamp, in.Timestamp)
	}
	if out.LogID != in.LogID {
		t.Errorf("LogID=%q want %q", out.LogID, in.LogID)
	}
}
