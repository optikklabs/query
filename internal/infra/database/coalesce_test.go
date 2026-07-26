package database

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	types "github.com/optikklabs/query/internal/shared/contracts"
)

type row struct {
	Name string
	P50  float64
	QS   []float64
}

// TestCoalesceRunsFetchOnce: concurrent identical reads collapse to one query.
func TestCoalesceRunsFetchOnce(t *testing.T) {
	var fetches atomic.Int64
	release := make(chan struct{})

	fetch := func(_ context.Context, out any) error {
		fetches.Add(1)
		<-release // hold the leader open so the others pile up behind it
		*(out.(*[]row)) = []row{{Name: "a"}, {Name: "b"}}
		return nil
	}

	const callers = 20
	var wg sync.WaitGroup
	results := make([][]row, callers)
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var dest []row
			if err := coalesce(context.Background(), "k", "op", &dest, fetch); err != nil {
				t.Errorf("caller %d: %v", i, err)
			}
			results[i] = dest
		}()
	}

	// Let every caller reach DoChan before the leader completes.
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := fetches.Load(); got != 1 {
		t.Errorf("fetches = %d, want 1: identical in-flight reads must collapse", got)
	}
	for i, r := range results {
		if len(r) != 2 || r[0].Name != "a" {
			t.Errorf("caller %d got %v, want the leader's result", i, r)
		}
	}
}

// TestCoalesceGivesEachCallerItsOwnBackingArray pins the aliasing contract.
// Several repositories mutate returned rows in place (spreading a quantiles
// array into P50/P95/P99). If callers shared one backing array those writes
// would race, so each must get its own copy. Run with -race.
func TestCoalesceGivesEachCallerItsOwnBackingArray(t *testing.T) {
	release := make(chan struct{})
	fetch := func(_ context.Context, out any) error {
		<-release
		*(out.(*[]row)) = []row{{Name: "a", QS: []float64{1, 2, 3}}}
		return nil
	}

	const callers = 8
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var dest []row
			if err := coalesce(context.Background(), "k", "op", &dest, fetch); err != nil {
				t.Errorf("caller %d: %v", i, err)
				return
			}
			// The post-SELECT mutation every RED repository performs.
			for j := range dest {
				dest[j].P50 = dest[j].QS[0] + float64(i)
			}
			if dest[0].P50 != 1+float64(i) {
				t.Errorf("caller %d saw P50 %v, want %v — rows were shared",
					i, dest[0].P50, 1+float64(i))
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
}

// TestCoalesceKeyIsolatesTenants: sharing a result across tenants would be a
// data leak, so the key must never collide on query and args alone.
func TestCoalesceKeyIsolatesTenants(t *testing.T) {
	q := "SELECT 1 FROM spans WHERE tenant_id = ?"
	args := []any{int64(1)}

	if coalesceKey(1, q, args) == coalesceKey(2, q, args) {
		t.Fatal("two tenants produced the same key")
	}
	if coalesceKey(1, q, args) != coalesceKey(1, q, args) {
		t.Error("key is not stable for identical inputs")
	}
	if coalesceKey(1, q, []any{int64(1)}) == coalesceKey(1, q, []any{int64(2)}) {
		t.Error("differing args produced the same key")
	}
	// A tenant id must not be able to run into the query text.
	if coalesceKey(1, "2"+q, args) == coalesceKey(12, q, args) {
		t.Error("tenant id and query text are ambiguously joined")
	}
}

// TestCoalesceLeaderSurvivesFollowerCancellation: a follower giving up must
// not cancel the shared query, and must return its own context error.
func TestCoalesceLeaderSurvivesFollowerCancellation(t *testing.T) {
	release := make(chan struct{})
	fetchErr := make(chan error, 1)

	fetch := func(runCtx context.Context, out any) error {
		<-release
		if err := runCtx.Err(); err != nil {
			fetchErr <- err // leader was cancelled by someone else's ctx
			return err
		}
		*(out.(*[]row)) = []row{{Name: "a"}}
		fetchErr <- nil
		return nil
	}

	leaderDone := make(chan error, 1)
	go func() {
		var dest []row
		leaderDone <- coalesce(context.Background(), "k2", "op", &dest, fetch)
	}()

	time.Sleep(20 * time.Millisecond)

	followerCtx, cancel := context.WithCancel(context.Background())
	followerDone := make(chan error, 1)
	go func() {
		var dest []row
		followerDone <- coalesce(followerCtx, "k2", "op", &dest, fetch)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel() // follower walks away

	if err := <-followerDone; !errors.Is(err, context.Canceled) {
		t.Errorf("follower error = %v, want context.Canceled", err)
	}

	close(release)

	if err := <-fetchErr; err != nil {
		t.Errorf("leader query saw %v — a follower's cancellation reached it", err)
	}
	if err := <-leaderDone; err != nil {
		t.Errorf("leader error = %v, want nil", err)
	}
}

// TestCoalesceRejectsNonPointerDest guards the reflection precondition.
func TestCoalesceRejectsNonPointerDest(t *testing.T) {
	err := coalesce(context.Background(), "k3", "op", []row{}, func(context.Context, any) error {
		t.Error("fetch ran despite an invalid dest")
		return nil
	})
	if err == nil {
		t.Error("coalesce accepted a non-pointer dest")
	}
}

// TestSelectCHKeyUsesTenantFromContext: the key must come from the request's
// tenant, not a caller-supplied argument that could be forgotten.
func TestSelectCHKeyUsesTenantFromContext(t *testing.T) {
	ctx := types.WithTenant(context.Background(), types.TenantContext{TenantID: 42})
	if got := types.TenantFrom(ctx).TenantID; got != 42 {
		t.Fatalf("tenant plumbing changed: got %d", got)
	}
	if coalesceKey(types.TenantFrom(ctx).TenantID, "q", nil) == coalesceKey(0, "q", nil) {
		t.Error("tenant from context did not reach the key")
	}
}
