// Package repogolden pins two things that survive no compiler check: the SQL
// repositories send to ClickHouse, and the operation labels they report it
// under.
//
// Regenerate with: go test ./internal/repogolden -update
package repogolden

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// opCall matches the op-label argument of a instrumented read, e.g.
//
//	dbutil.SelectCH(ctx, r.db, "latency.GetLatencyBySystem", &rows, query, ...)
var opCall = regexp.MustCompile(`(?:SelectCH|QueryRowCH)\(\s*[^,]+,\s*[^,]+,\s*"([^"]+)"`)

// TestOpLabelsGolden pins every ClickHouse operation label.
//
// These are Prometheus label values on optikk_db_query_duration_seconds and
// optikk_db_queries_total. Dashboards and alerts select on them, so they are a
// public contract: moving a repository between packages must not rename them,
// even though nothing in Go would complain if it did.
func TestOpLabelsGolden(t *testing.T) {
	var labels []string
	err := filepath.WalkDir("../..", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range opCall.FindAllSubmatch(src, -1) {
			labels = append(labels, string(m[1]))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) == 0 {
		t.Fatal("found no op labels — the scan regex no longer matches the call shape")
	}

	sort.Strings(labels)
	got := strings.Join(labels, "\n") + "\n"

	path := filepath.Join("testdata", "oplabels.golden.txt")
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
		t.Errorf("ClickHouse op labels changed — these are dashboard-visible metric "+
			"labels.\n--- want\n%s\n--- got\n%s", want, got)
	}
}
