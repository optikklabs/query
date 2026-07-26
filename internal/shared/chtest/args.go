// Package chtest holds test helpers for asserting on ClickHouse query
// arguments. It is imported only from _test.go files.
package chtest

import (
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// NamedArgs indexes bound arguments by bind name, so a test can assert on the
// value behind @services without depending on the order clauses were appended.
func NamedArgs(args []any) map[string]any {
	out := make(map[string]any, len(args))
	for _, a := range args {
		if nv, ok := a.(driver.NamedValue); ok {
			out[nv.Name] = nv.Value
		}
	}
	return out
}

// RenderValue prints a bound value with its Go type. time.Time is normalised
// to UTC: time.UnixMilli yields a local-zone value, so printing it directly
// would make a golden depend on the machine's timezone.
func RenderValue(v any) string {
	if t, ok := v.(time.Time); ok {
		return fmt.Sprintf("time.Time %s", t.UTC().Format(time.RFC3339Nano))
	}
	return fmt.Sprintf("%T %#v", v, v)
}
