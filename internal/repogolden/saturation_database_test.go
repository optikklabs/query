package repogolden

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dbexplorer "github.com/optikklabs/query/internal/modules/saturation/database/explorer"
	dbfilter "github.com/optikklabs/query/internal/modules/saturation/database/filter"
	dblatency "github.com/optikklabs/query/internal/modules/saturation/database/latency"
	dbquerydetail "github.com/optikklabs/query/internal/modules/saturation/database/querydetail"
	dbslowqueries "github.com/optikklabs/query/internal/modules/saturation/database/slowqueries"
	dbvolume "github.com/optikklabs/query/internal/modules/saturation/database/volume"
	"github.com/optikklabs/query/internal/shared/chtest"
)

// A fixed window so rollup-table choice and snapped bucket bounds are stable.
// 2026-01-02T02:06:40Z .. +6h, which lands on the 5-minute rollup.
const (
	tenantID       int64 = 7
	startMs        int64 = 1767319600000
	endMs          int64 = startMs + 6*3_600_000
	queryHash            = "a1b2c3"
	defaultRowCap        = 50
)

// dbFilters exercises every filter dimension at once, so a dropped filter
// argument shows as a missing bind rather than an identical query.
func dbFilters() dbfilter.Filters {
	return dbfilter.Filters{
		DBSystem:   []string{"postgresql"},
		Collection: []string{"orders"},
		Namespace:  []string{"app"},
		Server:     []string{"db.internal"},
	}
}

// TestSaturationDatabaseRepoSQL pins the SQL of every repository method in the
// saturation/database domain, which is about to be merged into one package.
// Two of these modules (latency, volume) had no test at all before this.
func TestSaturationDatabaseRepoSQL(t *testing.T) {
	ctx := context.Background()
	rec := &chtest.Recorder{}
	var b strings.Builder

	record := func(name string, call func()) {
		rec.Reset()
		call()
		fmt.Fprintf(&b, "=== %s\n%s\n", name, rec.Render())
	}

	lat := dblatency.NewRepository(rec)
	record("latency.GetLatencyBySystem", func() {
		_, _ = lat.GetLatencyBySystem(ctx, tenantID, startMs, endMs, dbFilters())
	})

	vol := dbvolume.NewRepository(rec)
	record("volume.GetOpsBySystem", func() {
		_, _ = vol.GetOpsBySystem(ctx, tenantID, startMs, endMs, dbFilters())
	})

	exp := dbexplorer.NewRepository(rec)
	record("explorer.GetSystemSummariesRaw", func() {
		_, _ = exp.GetSystemSummariesRaw(ctx, tenantID, startMs, endMs)
	})
	record("explorer.GetActiveConnectionsBySystem", func() {
		_, _ = exp.GetActiveConnectionsBySystem(ctx, tenantID, startMs, endMs)
	})

	slow := dbslowqueries.NewRepository(rec)
	record("slowqueries.GetSlowQueryPatterns", func() {
		_, _ = slow.GetSlowQueryPatterns(ctx, tenantID, startMs, endMs, dbFilters(), defaultRowCap)
	})

	qd := dbquerydetail.NewRepository(rec)
	record("querydetail.GetSummary", func() {
		_, _ = qd.GetSummary(ctx, tenantID, startMs, endMs, queryHash, dbFilters())
	})
	record("querydetail.GetServices", func() {
		_, _ = qd.GetServices(ctx, tenantID, startMs, endMs, queryHash, dbFilters())
	})
	record("querydetail.GetTimeseries", func() {
		_, _ = qd.GetTimeseries(ctx, tenantID, startMs, endMs, queryHash, dbFilters())
	})
	record("querydetail.GetExecutions", func() {
		_, _ = qd.GetExecutions(ctx, tenantID, startMs, endMs, queryHash, dbFilters(), defaultRowCap)
	})

	compareGolden(t, "saturation_database.golden.txt", b.String())
}

// compareGolden is shared by the per-domain repository snapshots.
func compareGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if got != string(want) {
		t.Errorf("repository SQL changed.\n--- want\n%s\n--- got\n%s", want, got)
	}
}
