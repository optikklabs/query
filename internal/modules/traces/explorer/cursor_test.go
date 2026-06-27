package explorer

import "testing"

// The trace cursor must preserve full nanosecond precision (the truncation bug)
// and carry span_id as the unique tiebreaker through encode/decode.
func TestTraceCursorRoundTrip(t *testing.T) {
	in := TraceCursor{StartNs: 1_700_000_000_123_456_789, SpanID: "abc123"}
	out, ok := DecodeCursor(in.Encode())
	if !ok {
		t.Fatal("decode failed")
	}
	if out.StartNs != in.StartNs {
		t.Errorf("StartNs=%d want %d (ns precision lost)", out.StartNs, in.StartNs)
	}
	if out.SpanID != in.SpanID {
		t.Errorf("SpanID=%q want %q", out.SpanID, in.SpanID)
	}
}
