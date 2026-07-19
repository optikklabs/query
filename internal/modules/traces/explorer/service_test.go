package explorer

import (
	"context"
	"testing"
	"time"
)

type stubRepo struct {
	gotLimit int
	rows     []traceIndexRowDTO
	aggs     map[string]traceAggRow
}

func (s *stubRepo) Query(_ context.Context, req QueryRequest) ([]traceIndexRowDTO, bool, error) {
	s.gotLimit = req.Limit
	return s.rows, false, nil
}

func (s *stubRepo) EnrichTraces(context.Context, int64, []string) (map[string]traceAggRow, error) {
	return s.aggs, nil
}
func (s *stubRepo) QueryFacets(context.Context, FacetsRequest) (Facets, error) {
	return Facets{}, nil
}
func (s *stubRepo) QueryTrend(context.Context, TrendRequest) ([]TrendBucket, error) {
	return nil, nil
}
func (s *stubRepo) SuggestAttribute(context.Context, int64, int64, int64, string, string, int) ([]Suggestion, error) {
	return nil, nil
}
func (s *stubRepo) SuggestScalar(context.Context, int64, int64, int64, string, string, int) ([]Suggestion, error) {
	return nil, nil
}

// A row must report its trace's facts, not its root span's. Previously
// span_count was hardcoded to 1, so every row claimed a single-span trace.
func TestServiceQuery_RowsReportTraceLevelFacts(t *testing.T) {
	start := time.Unix(1000, 0)
	repo := &stubRepo{
		rows: []traceIndexRowDTO{{
			TraceID:     "abc",
			StartTime:   start,
			DurationNs:  2_000_000, // root span: 2ms
			RootService: "gateway",
		}},
		aggs: map[string]traceAggRow{"abc": {
			TraceID:    "abc",
			SpanCount:  5,
			ErrorCount: 2,
			StartTime:  start,
			EndTime:    start.Add(50 * time.Millisecond),
			ServiceSet: []string{"gateway", "orders", "ledger"},
		}},
	}
	resp, err := NewService(repo).Query(context.Background(), QueryRequest{})
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Results[0]
	if got.SpanCount != 5 {
		t.Errorf("span_count = %d, want 5", got.SpanCount)
	}
	if got.ErrorCount != 2 || !got.HasError {
		t.Errorf("error_count = %d / has_error = %v, want 2 / true", got.ErrorCount, got.HasError)
	}
	if len(got.ServiceSet) != 3 {
		t.Errorf("service_set = %v, want 3 services", got.ServiceSet)
	}
	// Duration must be trace wall-clock, not the root span's 2ms.
	if got.DurationMs != 50 {
		t.Errorf("duration_ms = %v, want 50", got.DurationMs)
	}
	if got.EndMs <= got.StartMs {
		t.Errorf("end_ms %d must exceed start_ms %d", got.EndMs, got.StartMs)
	}
}

// A trace whose spans aged out of the aggregate still renders from its root.
func TestServiceQuery_FallsBackWhenAggregateMissing(t *testing.T) {
	repo := &stubRepo{
		rows: []traceIndexRowDTO{{TraceID: "gone", RootService: "gateway", DurationNs: 3_000_000}},
		aggs: map[string]traceAggRow{},
	}
	resp, err := NewService(repo).Query(context.Background(), QueryRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Results[0]; got.SpanCount != 1 || got.DurationMs != 3 {
		t.Errorf("fallback = span_count %d / duration %v, want 1 / 3", got.SpanCount, got.DurationMs)
	}
}

// The clamped limit must reach the repository, not just pageInfo.
func TestServiceQuery_ClampsLimitForRepo(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want int
	}{
		{"omitted defaults", 0, 50},
		{"over max clamped", 9999, 500},
		{"in range passes through", 100, 100},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &stubRepo{}
			svc := NewService(repo)
			resp, err := svc.Query(context.Background(), QueryRequest{Limit: c.in})
			if err != nil {
				t.Fatal(err)
			}
			if repo.gotLimit != c.want {
				t.Errorf("repo limit = %d, want %d", repo.gotLimit, c.want)
			}
			if resp.PageInfo.Limit != c.want {
				t.Errorf("pageInfo limit = %d, want %d", resp.PageInfo.Limit, c.want)
			}
		})
	}
}
