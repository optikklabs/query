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

func TestQueryWorkload(t *testing.T) {
	for _, tc := range []struct {
		path string
		want string
	}{
		{"/api/v1/traces/abc", workloadDetail},
		{"/api/v1/logs/abc", workloadDetail},
		{"/api/v1/logs/query", workloadExplorer},
		{"/api/v1/traces", workloadExplorer},
		{"/api/v1/traces/query", workloadExplorer},
		{"/api/v1/traces/facets", workloadExplorer},
		{"/api/v1/services/overview", workloadOverview},
		{"/api/v1/users", ""},
	} {
		r := httptest.NewRequest(http.MethodGet, tc.path, nil)
		if got := queryWorkload(r); got != tc.want {
			t.Errorf("queryWorkload(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}
