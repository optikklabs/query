package service

import (
	"testing"
	"time"

	"github.com/optikklabs/query/internal/modules/traces/repository"
)

func TestBuildErrorPathIsDeterministic(t *testing.T) {
	t0 := time.Unix(0, 0)
	// root -> a -> b, root -> c; b and c are leaves, b starts first.
	rows := []repository.TraceSpanRow{
		{SpanID: "root", Timestamp: t0},
		{SpanID: "a", ParentSpanID: "root", Timestamp: t0.Add(time.Millisecond)},
		{SpanID: "b", ParentSpanID: "a", Timestamp: t0.Add(2 * time.Millisecond)},
		{SpanID: "c", ParentSpanID: "root", Timestamp: t0.Add(3 * time.Millisecond)},
	}
	for range 20 {
		path := buildErrorPath(rows)
		if len(path) != 3 || path[0].SpanID != "root" || path[1].SpanID != "a" || path[2].SpanID != "b" {
			t.Fatalf("path = %+v, want root->a->b", path)
		}
	}
}

func TestBuildErrorPathEmpty(t *testing.T) {
	if path := buildErrorPath(nil); path == nil || len(path) != 0 {
		t.Fatalf("path = %#v, want empty non-nil", path)
	}
}

func TestGroupErrorsOrdersByCountThenType(t *testing.T) {
	rows := []repository.TraceSpanRow{
		{SpanID: "1", ExceptionType: "B"},
		{SpanID: "2", ExceptionType: "A"},
		{SpanID: "3", StatusMessage: "timeout"},
		{SpanID: "4", StatusMessage: "timeout"},
	}
	groups := groupErrors(rows)
	got := []string{groups[0].ExceptionType, groups[1].ExceptionType, groups[2].ExceptionType}
	want := []string{"timeout", "A", "B"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestResourceAttributesSkipsEmpty(t *testing.T) {
	got := resourceAttributes(&repository.SpanAttributeRow{ServiceName: "api", Host: "h1"})
	if len(got) != 2 || got["service.name"] != "api" || got["host.name"] != "h1" {
		t.Fatalf("resource attributes = %v", got)
	}
}
