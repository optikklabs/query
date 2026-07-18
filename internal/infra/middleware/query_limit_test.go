package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExpensiveQueryLimitSkipsNonTelemetryRoutes(t *testing.T) {
	called := false
	h := ExpensiveQueryLimit(1)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/users", nil))
	if !called {
		t.Fatal("non-telemetry route was not served")
	}
}

func TestIsExpensiveQuery(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/api/v1/traces", true},
		{"/api/v1/metrics/query", true},
		{"/api/v1/users", false},
		{"/health", false},
	} {
		r := httptest.NewRequest(http.MethodGet, tc.path, nil)
		if got := isExpensiveQuery(r); got != tc.want {
			t.Errorf("isExpensiveQuery(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
