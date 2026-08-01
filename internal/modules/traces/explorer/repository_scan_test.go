package explorer

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/optikklabs/query/internal/shared/spanfilter"
)

func TestTraceCursorIncludesFullSortingKey(t *testing.T) {
	want := TraceCursor{StartNs: 123, TraceID: "trace-b", SpanID: "span-c"}
	raw := want.Encode()
	if raw == "" {
		t.Fatal("non-zero cursor encoded as empty")
	}
	decoded, ok := DecodeCursor(raw)
	if !ok {
		t.Fatal("could not decode cursor")
	}
	if decoded != want {
		t.Fatalf("decoded cursor = %#v, want %#v", decoded, want)
	}

	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		t.Fatalf("cursor is not base64url: %v", err)
	}
	if !strings.Contains(string(payload), `"t":"trace-b"`) {
		t.Fatalf("cursor payload omits trace_id: %s", payload)
	}
}

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
