package httputil

import (
	"net/http/httptest"
	"testing"
)

func TestParseRequiredExplicitRange(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantStart  int64
		wantEnd    int64
		wantOK     bool
		wantStatus int
	}{
		{name: "valid pair", query: "?startTime=1000&endTime=2000", wantStart: 1000, wantEnd: 2000, wantOK: true, wantStatus: 200},
		{name: "both missing", wantStatus: 400},
		{name: "start only", query: "?startTime=1000", wantStatus: 400},
		{name: "end only", query: "?endTime=2000", wantStatus: 400},
		{name: "malformed", query: "?startTime=nope&endTime=2000", wantStatus: 400},
		{name: "non-positive", query: "?startTime=0&endTime=2000", wantStatus: 400},
		{name: "reversed", query: "?startTime=2000&endTime=1000", wantStatus: 400},
		{name: "equal", query: "?startTime=1000&endTime=1000", wantStatus: 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/trace"+tt.query, nil)
			recorder := httptest.NewRecorder()
			start, end, ok := ParseRequiredExplicitRange(recorder, req)
			if ok != tt.wantOK || start != tt.wantStart || end != tt.wantEnd {
				t.Fatalf("got (%d, %d, %v), want (%d, %d, %v)", start, end, ok, tt.wantStart, tt.wantEnd, tt.wantOK)
			}
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
		})
	}
}
