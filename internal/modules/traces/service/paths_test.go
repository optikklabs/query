package service

import (
	"testing"
	"time"

	"github.com/optikklabs/query/internal/modules/traces/models"
	"github.com/optikklabs/query/internal/modules/traces/repository"
)

func TestIsRootParentSpanID(t *testing.T) {
	cases := map[string]bool{
		"":                 true,
		"0000000000000000": true,
		"\x00\x00":         true,
		"abc123":           false,
	}
	for in, want := range cases {
		if got := isRootParentSpanID(in); got != want {
			t.Errorf("isRootParentSpanID(%q) = %v, want %v", in, got, want)
		}
	}
}

func spanIDs(spans []models.CriticalPathSpan) []string {
	out := make([]string, len(spans))
	for i, s := range spans {
		out[i] = s.SpanID
	}
	return out
}

// Critical path is the longest-duration root->leaf chain by subtree end time.
// Tree: R(dur100) -> {A(dur50), B(dur70) -> C(dur50)}. R's subtree ends at 100
// via itself, but the deepest chain is R->B->C (B outlasts A).
func TestBuildCriticalPath_LongestChain(t *testing.T) {
	rows := []repository.CriticalPathRow{
		{SpanID: "R", ParentSpanID: "", Timestamp: time.Unix(0, 0), DurationNano: 100, DurationMs: 0.1},
		{SpanID: "A", ParentSpanID: "R", Timestamp: time.Unix(0, 10), DurationNano: 50},
		{SpanID: "B", ParentSpanID: "R", Timestamp: time.Unix(0, 20), DurationNano: 70},
		{SpanID: "C", ParentSpanID: "B", Timestamp: time.Unix(0, 30), DurationNano: 50},
	}
	got := spanIDs(buildCriticalPath(rows))
	want := []string{"R", "B", "C"}
	if len(got) != len(want) {
		t.Fatalf("chain = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("chain = %v, want %v", got, want)
		}
	}
}

func TestPickBestChild_TieBreakByStart(t *testing.T) {
	nodes := map[string]*criticalNode{
		"early": {startNs: 10, subtreeEnd: 100},
		"late":  {startNs: 20, subtreeEnd: 100},
	}
	if got := pickBestChild(nodes, []string{"early", "late"}); got != "late" {
		t.Errorf("pickBestChild = %q, want late (higher start on tie)", got)
	}
}

func TestBuildErrorPath_RootToLeaf(t *testing.T) {
	rows := []repository.ErrorPathRow{
		{SpanID: "e2", ParentSpanID: "e1"},
		{SpanID: "e3", ParentSpanID: "e2"},
		{SpanID: "e1", ParentSpanID: ""},
	}
	got := buildErrorPath(rows)
	want := []string{"e1", "e2", "e3"}
	if len(got) != len(want) {
		t.Fatalf("path len = %d, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].SpanID != want[i] {
			t.Fatalf("path = %v, want %v", got, want)
		}
	}
}

func TestBuildErrorPath_Empty(t *testing.T) {
	got := buildErrorPath(nil)
	if got == nil || len(got) != 0 {
		t.Errorf("want non-nil empty slice, got %+v", got)
	}
}
