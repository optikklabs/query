// Package tracewindow is the single owner of "where in time does this trace
// live". Every trace-by-id read — spans and logs alike — bounds its scan with a
// Window resolved here, so no repository has to know how traces are located.
//
// The window comes from optikk.trace_index, which holds min(start)/max(end)
// across every span of a trace. Bounding spans by that range is complete by
// construction: a span cannot start before the trace's earliest span or after
// its latest span ends. Log reads use Days instead, because a log can sit just
// outside the span lifetime.
package tracewindow

import (
	"context"
	"log/slog"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	dbutil "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/metrics"
)

// logsRetention mirrors the DELETE TTL on optikk.logs (05_logs.sql). It bounds
// the log reads that have no trace window to narrow with, so an unresolved
// trace scans retention rather than the whole table.
const logsRetention = 15 * 24 * time.Hour

// Window is a trace's lifetime, inclusive at both ends.
type Window struct {
	Start time.Time
	End   time.Time
}

// Resolve locates a trace's span lifetime. ok is false when the trace has no
// index rows, which means it has no spans at all.
func Resolve(ctx context.Context, db clickhouse.Conn, tenantID int64, traceID string) (w Window, ok bool, err error) {
	// count() is what detects the miss, not a NULL check: min/max over an empty
	// set return the DateTime64 default (1970), which is a valid-looking window
	// that silently matches no partitions.
	const query = `
		SELECT min(start_ts) AS start_ts,
		       max(end_ts)   AS end_ts,
		       count()       AS rows_found
		FROM optikk.trace_index
		WHERE tenant_id = @tenantID AND trace_id = @traceID`

	var row struct {
		Start     time.Time `ch:"start_ts"`
		End       time.Time `ch:"end_ts"`
		RowsFound uint64    `ch:"rows_found"`
	}
	if err := dbutil.QueryRowCH(dbutil.ExplorerCtx(ctx), db, "tracewindow.Resolve", &row, query,
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("traceID", traceID),
	); err != nil {
		return Window{}, false, err
	}
	if row.RowsFound == 0 {
		// Counted and logged here so every caller reports a miss identically.
		metrics.TraceIndexMiss.Inc()
		slog.WarnContext(ctx, "tracewindow: trace not in index",
			slog.Int64("tenant_id", tenantID),
			slog.String("trace_id", traceID),
		)
		return Window{}, false, nil
	}
	return Window{Start: row.Start, End: row.End}, true, nil
}

// Days widens w to the day-partition boundaries it touches. Log reads use this
// rather than the exact span lifetime: a log is legitimately emitted just
// outside it — before the first span opens, or after the root span closes — and
// an exact bound would drop it. Both optikk.spans and optikk.logs are
// PARTITION BY toYYYYMMDD(timestamp), so this still prunes to the one or two
// partitions the trace touches.
func (w Window) Days() Window {
	return Window{
		Start: w.Start.UTC().Truncate(24 * time.Hour),
		End:   w.End.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour),
	}
}

// RetentionFallback is the window for log reads whose trace could not be
// resolved. Logs-only traces are real — every span may have been sampled out —
// so those reads must stay bounded rather than return nothing.
func RetentionFallback(now time.Time) Window {
	return Window{Start: now.Add(-logsRetention), End: now}
}

// Clamp intersects w with a caller-supplied millisecond range, for explorer
// filters that already carry a UI time range. An empty intersection is returned
// as-is; the caller decides whether that means "no results" or "leave alone".
func (w Window) Clamp(startMs, endMs int64) (int64, int64) {
	return max(startMs, w.Start.UnixMilli()), min(endMs, w.End.UnixMilli())
}

// NarrowSpanRange tightens an explorer's UI range to the trace's own lifetime
// when a trace_id filter is set, so a pasted trace id scans its own partitions
// instead of the whole range. ok is false when the trace has no spans, meaning
// the caller should return no results without querying. A blank traceID leaves
// the range untouched.
func NarrowSpanRange(ctx context.Context, db clickhouse.Conn, tenantID int64, traceID string, startMs, endMs int64) (int64, int64, bool, error) {
	if traceID == "" {
		return startMs, endMs, true, nil
	}
	w, ok, err := Resolve(ctx, db, tenantID, traceID)
	if err != nil || !ok {
		return startMs, endMs, false, err
	}
	s, e := w.Clamp(startMs, endMs)
	return s, e, s <= e, nil
}

// NarrowLogRange is NarrowSpanRange for logs: rounded out to day partitions,
// and a miss leaves the range untouched rather than returning nothing, because
// a trace whose spans were all sampled out still has logs.
func NarrowLogRange(ctx context.Context, db clickhouse.Conn, tenantID int64, traceID string, startMs, endMs int64) (int64, int64, error) {
	if traceID == "" {
		return startMs, endMs, nil
	}
	w, ok, err := Resolve(ctx, db, tenantID, traceID)
	if err != nil || !ok {
		return startMs, endMs, err
	}
	s, e := w.Days().Clamp(startMs, endMs)
	if s > e {
		return startMs, endMs, nil
	}
	return s, e, nil
}

// Args returns the named args every trace-by-id query takes, mirroring
// chargs.RangeArgs for range-scoped readers.
func Args(tenantID int64, traceID string, w Window) []any {
	return []any{
		clickhouse.Named("tenantID", uint32(tenantID)),
		clickhouse.Named("traceID", traceID),
		clickhouse.Named("start", w.Start),
		clickhouse.Named("end", w.End),
	}
}
