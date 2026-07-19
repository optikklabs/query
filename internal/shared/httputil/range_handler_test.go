package httputil

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// The envelope is a contract: payload always at data, previous period at the
// comparison sibling. Nesting a second {data:…} under data silently zeroed the
// Overview request-rate chart, so the shape is pinned here.
func TestHandleComparableRangeQueryEnvelope(t *testing.T) {
	series := func(_ context.Context, _, startMs, _ int64) (any, error) {
		return []map[string]any{{"timestamp": startMs, "requestCount": 1}}, nil
	}

	tests := []struct {
		name           string
		query          string
		wantCompared   bool
		wantCmpStartMs float64
	}{
		{
			name:  "without compareTo the comparison sibling is absent",
			query: "?startTime=2000&endTime=3000",
		},
		{
			name:           "with compareTo the previous period lands beside data",
			query:          "?startTime=2000&endTime=3000&compareTo=previous_period",
			wantCompared:   true,
			wantCmpStartMs: 1000,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/"+test.query, nil)

			HandleComparableRangeQuery(rec, req, "failed", series)

			if rec.Code != 200 {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}

			var envelope map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			rows, ok := envelope["data"].([]any)
			if !ok {
				t.Fatalf("data is not the payload array: %#v", envelope["data"])
			}
			if got := rows[0].(map[string]any)["timestamp"]; got != float64(2000) {
				t.Errorf("data holds the primary period, got timestamp %v", got)
			}

			cmp, hasCmp := envelope["comparison"]
			if hasCmp != test.wantCompared {
				t.Fatalf("comparison present = %v, want %v", hasCmp, test.wantCompared)
			}
			if !test.wantCompared {
				return
			}
			cmpRows := cmp.([]any)
			if got := cmpRows[0].(map[string]any)["timestamp"]; got != test.wantCmpStartMs {
				t.Errorf("comparison start = %v, want %v", got, test.wantCmpStartMs)
			}
		})
	}
}
