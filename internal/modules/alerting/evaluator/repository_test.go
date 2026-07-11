package evaluator

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Integration tests for the MySQL work-claiming lease. They need a live
// database and are skipped unless OPTIKK_TEST_MYSQL_DSN is set, e.g.
// OPTIKK_TEST_MYSQL_DSN="root:password@tcp(127.0.0.1:3306)/optikk?parseTime=true"
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("OPTIKK_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("OPTIKK_TEST_MYSQL_DSN not set; skipping MySQL integration test")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping mysql: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// seedMonitor inserts an active monitor with a due state row and returns
// its id. Rows are cleaned up when the test finishes.
func seedMonitor(t *testing.T, db *sql.DB, due time.Time) int64 {
	t.Helper()
	res, err := db.Exec(`
		INSERT INTO optikk.monitors
		  (tenant_id, name, type, scope_json, query_json, conditions_json, notify_json)
		VALUES (999999, 'claim-test', 'metric', '{}', '{}', '{}', '{}')`)
	if err != nil {
		t.Fatalf("insert monitor: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("monitor id: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO optikk.monitor_state (monitor_id, next_evaluation_at)
		VALUES (?, ?)`, id, due); err != nil {
		t.Fatalf("insert state: %v", err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM optikk.monitor_state WHERE monitor_id = ?`, id)
		db.Exec(`DELETE FROM optikk.monitors WHERE id = ?`, id)
	})
	return id
}

func TestClaimDueIsExclusiveAcrossClaimers(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	idA := seedMonitor(t, db, now.Add(-time.Minute))
	idB := seedMonitor(t, db, now.Add(-time.Minute))

	gotA, err := repo.ClaimDue(ctx, "claimer-a", now, 500)
	if err != nil {
		t.Fatalf("claimer-a: %v", err)
	}
	gotB, err := repo.ClaimDue(ctx, "claimer-b", now, 500)
	if err != nil {
		t.Fatalf("claimer-b: %v", err)
	}

	seen := map[int64]string{}
	for _, d := range gotA {
		seen[d.Monitor.ID] = "a"
	}
	for _, d := range gotB {
		if owner, dup := seen[d.Monitor.ID]; dup {
			t.Errorf("monitor %d claimed by both %q and b", d.Monitor.ID, owner)
		}
	}
	for _, id := range []int64{idA, idB} {
		if _, ok := seen[id]; !ok {
			t.Errorf("monitor %d not claimed by claimer-a", id)
		}
	}
}

func TestClaimDueReclaimsExpiredLease(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	id := seedMonitor(t, db, now.Add(-time.Minute))
	if _, err := db.Exec(`
		UPDATE optikk.monitor_state
		   SET claimed_by = 'crashed-replica', claimed_until = ?
		 WHERE monitor_id = ?`, now.Add(-time.Second), id); err != nil {
		t.Fatalf("stamp expired lease: %v", err)
	}

	got, err := repo.ClaimDue(ctx, "claimer-c", now, 500)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	found := false
	for _, d := range got {
		if d.Monitor.ID == id {
			found = true
		}
	}
	if !found {
		t.Error("expired lease was not reclaimed")
	}
}

func TestUpdateStateReleasesClaim(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	id := seedMonitor(t, db, now.Add(-time.Minute))
	if _, err := repo.ClaimDue(ctx, "claimer-d", now, 500); err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}

	err := repo.UpdateState(ctx, UpdateStateArgs{
		MonitorID:          id,
		PrevStatus:         "no_data",
		NewStatus:          "ok",
		LastEvaluatedAt:    now,
		NextEvaluationAt:   now.Add(5 * time.Minute),
		IncrementEvalCount: true,
	})
	if err != nil {
		t.Fatalf("UpdateState: %v", err)
	}

	var claimedBy sql.NullString
	if err := db.QueryRow(`
		SELECT claimed_by FROM optikk.monitor_state WHERE monitor_id = ?`, id,
	).Scan(&claimedBy); err != nil {
		t.Fatalf("read claim: %v", err)
	}
	if claimedBy.Valid {
		t.Errorf("claimed_by = %q after UpdateState, want NULL", claimedBy.String)
	}
}
