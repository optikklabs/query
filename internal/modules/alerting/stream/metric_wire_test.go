package stream

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestDecodeMetricEvent(t *testing.T) {
	// tenant_id=9, metric_name="cpu", fingerprint=4, timestamp_ns=5,
	// value=12.5. These are the field numbers emitted by ingest's MetricRow.
	b := []byte{0x08, 0x09, 0x12, 0x03, 'c', 'p', 'u', 0x40, 0x04, 0x48, 0x05, 0x51}
	bits := make([]byte, 8)
	binary.LittleEndian.PutUint64(bits, math.Float64bits(12.5))
	b = append(b, bits...)
	got, err := decodeMetricEvent(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.TenantID != 9 || got.MetricName != "cpu" || got.Fingerprint != 4 || got.TimestampNs != 5 || got.Value != 12.5 {
		t.Fatalf("decoded = %#v", got)
	}
}

func TestDecodeMetricEventRejectsTruncatedData(t *testing.T) {
	if _, err := decodeMetricEvent([]byte{0x51, 1}); err == nil {
		t.Fatal("expected error")
	}
}
