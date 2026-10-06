package chargs

import (
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Millis binds an epoch-millisecond bound as DateTime64(3). Never bind a
// time.Time with clickhouse.Named: it is sent as a second-precision DateTime,
// which truncates the bound, and against a DateTime64 key column ClickHouse's
// primary-key analysis can then drop rows inside the final second.
func Millis(name string, ms int64) driver.NamedDateValue {
	return clickhouse.DateNamed(name, time.UnixMilli(ms), clickhouse.MilliSeconds)
}

// Nanos binds a full-precision timestamp, for keyset cursors on DateTime64(9)
// columns.
func Nanos(name string, t time.Time) driver.NamedDateValue {
	return clickhouse.DateNamed(name, t, clickhouse.NanoSeconds)
}

func RangeArgs(tenantID, startMs, endMs int64) []any {
	return []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		Millis("start", startMs),
		Millis("end", endMs),
	}
}

func WithMetricNames(args []any, names []string) []any {
	return append(args, clickhouse.Named("metricNames", names))
}
