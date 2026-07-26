package chtest

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Recorder is a driver.Conn that executes nothing and records the SQL and
// bound arguments it was asked to run.
//
// It exists so repository methods can be exercised without a ClickHouse: the
// query text and its binds are built entirely in Go before the call, so they
// are fully observable at this seam. Reads return no rows, which every
// repository already handles — that is the shape of an empty time range.
type Recorder struct {
	mu    sync.Mutex
	calls []Call
}

// Call is one recorded read.
type Call struct {
	Method string // "Select" or "QueryRow"
	Query  string
	Args   []any
}

func (r *Recorder) record(method, query string, args []any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, Call{Method: method, Query: query, Args: args})
}

// Calls returns the reads recorded so far, in order.
func (r *Recorder) Calls() []Call {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Call(nil), r.calls...)
}

// Reset drops recorded calls so one Recorder can serve several cases.
func (r *Recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = nil
}

// Render formats every recorded call: SQL with whitespace collapsed, then
// binds sorted by name. Sorted because ClickHouse resolves named binds by
// name, so emission order is not part of the contract.
func (r *Recorder) Render() string {
	var b strings.Builder
	for i, c := range r.Calls() {
		fmt.Fprintf(&b, "  [%d] %s: %s\n", i, c.Method, strings.Join(strings.Fields(c.Query), " "))
		lines := make([]string, 0, len(c.Args))
		for _, a := range c.Args {
			nv, ok := a.(driver.NamedValue)
			if !ok {
				lines = append(lines, fmt.Sprintf("      (positional) %s", RenderValue(a)))
				continue
			}
			lines = append(lines, fmt.Sprintf("      @%s = %s", nv.Name, RenderValue(nv.Value)))
		}
		sort.Strings(lines)
		for _, l := range lines {
			b.WriteString(l + "\n")
		}
	}
	return b.String()
}

func (r *Recorder) Select(_ context.Context, _ any, query string, args ...any) error {
	r.record("Select", query, args)
	return nil // no rows: dest is left at its zero value
}

func (r *Recorder) QueryRow(_ context.Context, query string, args ...any) driver.Row {
	r.record("QueryRow", query, args)
	return noRow{}
}

func (r *Recorder) Query(_ context.Context, query string, args ...any) (driver.Rows, error) {
	r.record("Query", query, args)
	return nil, fmt.Errorf("chtest: Query is not supported; repositories under test use Select")
}

// Everything below is required by driver.Conn and unused by read paths.

func (r *Recorder) Exec(context.Context, string, ...any) error { return nil }
func (r *Recorder) PrepareBatch(context.Context, string, ...driver.PrepareBatchOption) (driver.Batch, error) {
	return nil, fmt.Errorf("chtest: PrepareBatch is not supported")
}
func (r *Recorder) AsyncInsert(context.Context, string, bool, ...any) error { return nil }
func (r *Recorder) Ping(context.Context) error                              { return nil }
func (r *Recorder) Stats() driver.Stats                                     { return driver.Stats{} }
func (r *Recorder) Close() error                                            { return nil }
func (r *Recorder) Contributors() []string                                  { return nil }
func (r *Recorder) ServerVersion() (*driver.ServerVersion, error) {
	return &driver.ServerVersion{}, nil
}

// noRow reports "zero rows", which QueryRowCH maps to an empty result.
type noRow struct{}

func (noRow) Err() error           { return sql.ErrNoRows }
func (noRow) Scan(...any) error    { return sql.ErrNoRows }
func (noRow) ScanStruct(any) error { return sql.ErrNoRows }
