package explorer

import "testing"

func validQueryRequest() FEQueryRequest {
	return FEQueryRequest{
		StartTime: 1,
		EndTime:   60_001,
		Step:      "1m",
		Queries: []FEMetricQuery{{
			ID: "a", MetricName: "requests", Aggregation: "sum",
		}},
	}
}

func TestValidateQueryRequest(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*FEQueryRequest)
	}{
		{name: "reversed range", mutate: func(r *FEQueryRequest) { r.EndTime = r.StartTime }},
		{name: "unsupported step", mutate: func(r *FEQueryRequest) { r.Step = "2m" }},
		{name: "duplicate id", mutate: func(r *FEQueryRequest) { r.Queries = append(r.Queries, r.Queries[0]) }},
		{name: "too many groups", mutate: func(r *FEQueryRequest) { r.Queries[0].GroupBy = []string{"a", "b", "c", "d"} }},
		{name: "unsupported aggregation", mutate: func(r *FEQueryRequest) { r.Queries[0].Aggregation = "median" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := validQueryRequest()
			test.mutate(&req)
			if err := validateQueryRequest(req); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if err := validateQueryRequest(validQueryRequest()); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
}
