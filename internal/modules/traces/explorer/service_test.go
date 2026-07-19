package explorer

import (
	"context"
	"testing"
)

type stubRepo struct {
	gotLimit int
}

func (s *stubRepo) Query(_ context.Context, req QueryRequest) ([]traceIndexRowDTO, bool, error) {
	s.gotLimit = req.Limit
	return nil, false, nil
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
