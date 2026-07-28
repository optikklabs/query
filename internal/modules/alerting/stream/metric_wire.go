package stream

import (
	"encoding/binary"
	"fmt"
	"math"
)

type MetricEvent struct {
	TenantID    int64
	MetricName  string
	Fingerprint uint64
	TimestampNs int64
	Value       float64
}

func decodeMetricEvent(b []byte) (MetricEvent, error) {
	var out MetricEvent
	for len(b) > 0 {
		key, n := readVarint(b)
		if n <= 0 {
			return MetricEvent{}, fmt.Errorf("metric protobuf: invalid field key")
		}
		b = b[n:]
		field, wire := int(key>>3), int(key&7)
		switch wire {
		case 0:
			v, used := readVarint(b)
			if used <= 0 {
				return MetricEvent{}, fmt.Errorf("metric protobuf: invalid varint field %d", field)
			}
			b = b[used:]
			switch field {
			case 1:
				out.TenantID = int64(v)
			case 8:
				out.Fingerprint = v
			case 9:
				out.TimestampNs = int64(v)
			}
		case 1:
			if len(b) < 8 {
				return MetricEvent{}, fmt.Errorf("metric protobuf: truncated fixed64 field %d", field)
			}
			if field == 10 {
				out.Value = math.Float64frombits(binary.LittleEndian.Uint64(b[:8]))
			}
			b = b[8:]
		case 2:
			l, used := readVarint(b)
			if used <= 0 || l > uint64(len(b)-used) {
				return MetricEvent{}, fmt.Errorf("metric protobuf: invalid length field %d", field)
			}
			value := b[used : used+int(l)]
			if field == 2 {
				out.MetricName = string(value)
			}
			b = b[used+int(l):]
		case 5:
			if len(b) < 4 {
				return MetricEvent{}, fmt.Errorf("metric protobuf: truncated fixed32 field %d", field)
			}
			b = b[4:]
		default:
			return MetricEvent{}, fmt.Errorf("metric protobuf: unsupported wire type %d", wire)
		}
	}
	if out.TenantID <= 0 || out.MetricName == "" || out.TimestampNs <= 0 || math.IsNaN(out.Value) || math.IsInf(out.Value, 0) {
		return MetricEvent{}, fmt.Errorf("metric protobuf: missing or invalid required alert fields")
	}
	return out, nil
}

func readVarint(b []byte) (uint64, int) {
	var v uint64
	for i, c := range b {
		if i == 10 {
			return 0, -1
		}
		v |= uint64(c&0x7f) << (7 * i)
		if c < 0x80 {
			return v, i + 1
		}
	}
	return 0, 0
}
