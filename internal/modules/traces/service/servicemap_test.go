package service

import (
	"testing"

	"github.com/optikklabs/query/internal/modules/traces/repository"
)

// nodeAggsFromSpans aggregates per service: counts, errors, and mean duration
// (accumulated in P50Ms then divided by request count). Empty service skipped.
func TestNodeAggsFromSpans(t *testing.T) {
	rows := []repository.ServiceMapSpanRow{
		{SpanID: "1", ServiceName: "a", DurationMs: 10},
		{SpanID: "2", ServiceName: "a", DurationMs: 30, HasError: true},
		{SpanID: "3", ServiceName: "", DurationMs: 99},
	}
	out := nodeAggsFromSpans(rows)
	if len(out) != 1 {
		t.Fatalf("got %d nodes, want 1: %+v", len(out), out)
	}
	a := out[0]
	if a.Service != "a" || a.RequestCount != 2 || a.ErrorCount != 1 || a.P50Ms != 20 {
		t.Errorf("agg = %+v, want a/2/1/mean20", a)
	}
}

func TestEdgeAggsFromSpans(t *testing.T) {
	rows := []repository.ServiceMapSpanRow{
		{SpanID: "p", ParentSpanID: "", ServiceName: "a"},
		{SpanID: "c1", ParentSpanID: "p", ServiceName: "b", DurationMs: 10, HasError: true},
		{SpanID: "c2", ParentSpanID: "p", ServiceName: "b", DurationMs: 30},
		{SpanID: "c3", ParentSpanID: "p", ServiceName: "a"},
	}
	out := edgeAggsFromSpans(rows)
	if len(out) != 1 {
		t.Fatalf("got %d edges, want 1: %+v", len(out), out)
	}
	e := out[0]
	if e.Source != "a" || e.Target != "b" || e.CallCount != 2 || e.ErrorCount != 1 || e.P50Ms != 20 {
		t.Errorf("edge = %+v, want a->b/2/1/mean20", e)
	}
}

func TestErrorGroupKey(t *testing.T) {
	cases := []struct {
		row  repository.TraceErrorRow
		want string
	}{
		{repository.TraceErrorRow{ExceptionType: "NPE", StatusMessage: "msg"}, "NPE"},
		{repository.TraceErrorRow{StatusMessage: "boom"}, "boom"},
		{repository.TraceErrorRow{}, "UnknownError"},
	}
	for _, c := range cases {
		if got := errorGroupKey(c.row); got != c.want {
			t.Errorf("errorGroupKey(%+v) = %q, want %q", c.row, got, c.want)
		}
	}
}

func TestGroupErrors_SortedByCount(t *testing.T) {
	rows := []repository.TraceErrorRow{
		{SpanID: "1", ExceptionType: "A"},
		{SpanID: "2", ExceptionType: "B"},
		{SpanID: "3", ExceptionType: "B"},
		{SpanID: "4", ExceptionType: "B"},
	}
	out := groupErrors(rows)
	if len(out) != 2 {
		t.Fatalf("got %d groups, want 2", len(out))
	}
	if out[0].ExceptionType != "B" || out[0].Count != 3 {
		t.Errorf("first group = %+v, want B/3", out[0])
	}
	if out[1].ExceptionType != "A" || out[1].Count != 1 {
		t.Errorf("second group = %+v, want A/1", out[1])
	}
}
