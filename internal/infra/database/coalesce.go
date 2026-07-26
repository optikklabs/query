package database

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/optikklabs/query/internal/infra/metrics"
)

// chGroup coalesces identical in-flight ClickHouse reads. A dashboard with a
// dozen panels on the same range, or many viewers of one tenant's dashboard,
// otherwise issues the same query many times over.
//
// This dedups only what is genuinely in flight: a caller arriving after the
// leader finished runs its own query, so nothing is ever served stale.
var chGroup singleflight.Group

// leaderTimeout bounds a query once it is detached from the request that
// started it. It sits above every per-budget max_execution_time so the
// server-side limit is what actually fires.
const leaderTimeout = 90 * time.Second

// coalesceKey identifies an identical read. tenantID leads and is mandatory:
// sharing a result across tenants is a data leak, not an optimisation.
func coalesceKey(tenantID int64, query string, args []any) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d\x00%s", tenantID, query)
	for _, a := range args {
		fmt.Fprintf(&b, "\x00%#v", a)
	}
	return b.String()
}

// coalesce runs fetch at most once per identical in-flight key and gives every
// caller its own copy of the result.
//
// Aliasing contract: the top-level result is copied per caller, so repositories
// that mutate returned rows in place (several spread a quantiles array into
// P50/P95/P99 columns) cannot race each other. Reference-typed fields *inside*
// a row — maps, nested slices — stay shared and must be treated as read-only
// once fetched.
func coalesce(ctx context.Context, key, op string, dest any, fetch func(context.Context, any) error) error {
	destPtr := reflect.ValueOf(dest)
	if destPtr.Kind() != reflect.Pointer || destPtr.IsNil() {
		return fmt.Errorf("coalesce: dest must be a non-nil pointer, got %T", dest)
	}
	elemType := destPtr.Type().Elem()

	// Only the leader's closure ever runs, so this stays false for followers.
	// The channel receive below happens-after fn returns, so reading it is safe.
	leader := false
	result := chGroup.DoChan(key, func() (any, error) {
		leader = true

		// Detached from the originating request: the first caller in must not
		// cancel the query for everyone queued behind it.
		runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), leaderTimeout)
		defer cancel()

		fresh := reflect.New(elemType)
		if err := fetch(runCtx, fresh.Interface()); err != nil {
			return nil, err
		}
		return fresh.Elem().Interface(), nil
	})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-result:
		if !leader {
			metrics.DBQueriesCoalesced.WithLabelValues(op).Inc()
		}
		if res.Err != nil {
			return res.Err
		}
		assign(destPtr.Elem(), reflect.ValueOf(res.Val))
		return nil
	}
}

// assign copies src into dst, giving slices a fresh backing array so one
// caller's in-place row edits never reach another's.
func assign(dst, src reflect.Value) {
	if src.Kind() == reflect.Slice && !src.IsNil() {
		out := reflect.MakeSlice(src.Type(), src.Len(), src.Len())
		reflect.Copy(out, src)
		dst.Set(out)
		return
	}
	dst.Set(src)
}
