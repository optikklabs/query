package explorer

import (
	"strings"
	"testing"

	"github.com/optikklabs/query/internal/shared/spanfilter"
)

func TestSpanMatchCTEIsExact(t *testing.T) {
	c := spanfilter.BuildClauses(spanfilter.Filters{
		TenantID:   1,
		StartMs:    0,
		EndMs:      3_600_000,
		SpanKinds:  []string{"SERVER"},
		Operations: []string{"GET /checkout"},
	})
	if !c.HasSpanMatch() {
		t.Fatal("fixture produced no span-level predicate")
	}

	prefix, _, where := buildScanClauses(c)
	if strings.Contains(prefix, "LIMIT") {
		t.Errorf("span-match CTE silently truncates aggregate inputs:\n%s", prefix)
	}
	if !strings.Contains(where, "trace_id IN matched") {
		t.Errorf("outer query lost the match filter: %s", where)
	}
}

// No span-level predicate means no CTE at all.
func TestNoSpanMatchEmitsNoCTE(t *testing.T) {
	c := spanfilter.BuildClauses(spanfilter.Filters{TenantID: 1, StartMs: 0, EndMs: 3_600_000})
	prefix, _, where := buildScanClauses(c)
	if prefix != "" {
		t.Errorf("want no CTE, got:\n%s", prefix)
	}
	if strings.Contains(where, "matched") {
		t.Errorf("want no match filter, got: %s", where)
	}
}
