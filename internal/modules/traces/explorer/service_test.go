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

func TestServiceEnrichTraces_ReportsTraceLevelFacts(t *testing.T) {
	start := time.Unix(1000, 0)
	repo := &stubRepo{
		aggs: map[string]traceAggRow{"abc": {
			TraceID:    "abc",
			SpanCount:  5,
			ErrorCount: 2,
			StartTime:  start,
			EndTime:    start.Add(50 * time.Millisecond),
			ServiceSet: []string{"gateway", "orders", "ledger"},
		}},
	}
	resp, err := NewService(repo).EnrichTraces(context.Background(), 1, []string{"abc"})
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Enrichments["abc"]
	if got.SpanCount != 5 {
		t.Errorf("span_count = %d, want 5", got.SpanCount)
	}
	if got.ErrorCount != 2 || !got.HasError {
		t.Errorf("error_count = %d / has_error = %v, want 2 / true", got.ErrorCount, got.HasError)
	}
	if len(got.ServiceSet) != 3 {
		t.Errorf("service_set = %v, want 3 services", got.ServiceSet)
	}
	if got.DurationMs != 50 {
		t.Errorf("duration_ms = %v, want 50", got.DurationMs)
	}
	if got.EndMs <= got.StartMs {
		t.Errorf("end_ms %d must exceed start_ms %d", got.EndMs, got.StartMs)
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
