package redfleet

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	database "github.com/optikklabs/query/internal/infra/database"
	"github.com/optikklabs/query/internal/infra/timebucket"
	"github.com/optikklabs/query/internal/shared/chargs"
)

// Reconciles the repository's own rows against an independent aggregate over
// the same window, per bucket. Parsing proves nothing about which measure fed
// which field, and the alias bug returned rows while resolving wrongly.
//
// Local only — skipped unless CHVERIFY_DSN is set.
func TestReconcileRequestAndErrorRateAgainstClickHouse(t *testing.T) {
	dsn := os.Getenv("CHVERIFY_DSN")
	if dsn == "" {
		t.Skip("set CHVERIFY_DSN to run")
	}
	conn, err := database.OpenClickHouseConn(dsn, 4, 2)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	f := REDFilters{
		TenantID: 1,
		StartMs:  now.Add(-6 * time.Hour).UnixMilli(),
		EndMs:    now.UnixMilli(),
	}

	got, err := NewRepository(conn).GetRequestAndErrorRateTimeSeries(ctx, f)
	if err != nil {
		t.Fatalf("repository: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("no rows returned; cannot reconcile against an empty result")
	}

	// Independent aggregate: no aliases at all, so it cannot share the bug.
	expected := map[int64][2]uint64{}
	rows, err := conn.Query(database.OverviewCtx(ctx), `
		SELECT `+timebucket.DisplayGrainSQL(f.EndMs-f.StartMs)+`,
		       sum(request_count),
		       sumIf(request_count, status_code_string = 'ERROR')
		FROM `+timebucket.SpanStatsRollup(f.EndMs-f.StartMs)+`
		PREWHERE tenant_id = @tenantID AND timestamp BETWEEN @start AND @end
		GROUP BY 1`,
		chargs.RollupRangeArgs(f.TenantID, f.StartMs, f.EndMs)...)
	if err != nil {
		t.Fatalf("control query: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var bucket time.Time
		var req, errs uint64
		if err := rows.Scan(&bucket, &req, &errs); err != nil {
			t.Fatalf("scan: %v", err)
		}
		expected[bucket.Unix()] = [2]uint64{req, errs}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	var totalReq, totalErr uint64
	for _, r := range got {
		want, ok := expected[r.BucketAt.Unix()]
		if !ok {
			t.Errorf("bucket %s returned by repository but absent from control aggregate",
				r.BucketAt.Format(time.RFC3339))
			continue
		}
		if r.RequestCount != want[0] || r.ErrorCount != want[1] {
			t.Errorf("bucket %s: got requests=%d errors=%d, want requests=%d errors=%d",
				r.BucketAt.Format(time.RFC3339), r.RequestCount, r.ErrorCount, want[0], want[1])
		}
		totalReq += r.RequestCount
		totalErr += r.ErrorCount
	}
	if len(got) != len(expected) {
		t.Errorf("bucket count: repository %d, control %d", len(got), len(expected))
	}
	// Errors must be a strict subset of requests, per bucket and overall.
	if totalErr > totalReq {
		t.Errorf("errors %d exceed requests %d", totalErr, totalReq)
	}
	fmt.Printf("reconciled %d buckets: requests=%d errors=%d\n", len(got), totalReq, totalErr)
}
