package middleware

import (
	"net/http"
	"strings"
)

// ExpensiveQueryLimit bounds concurrent telemetry reads per API pod. It waits
// for capacity so request cancellation and the normal HTTP timeout remain the
// single source of timeout behavior.
func ExpensiveQueryLimit(max int) func(http.Handler) http.Handler {
	if max <= 0 {
		max = 1
	}
	sem := make(chan struct{}, max)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isExpensiveQuery(r) {
				next.ServeHTTP(w, r)
				return
			}
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-r.Context().Done():
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isExpensiveQuery(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		return false
	}
	for _, prefix := range expensiveQueryPrefixes {
		if strings.HasPrefix(r.URL.Path, prefix) {
			return true
		}
	}
	return false
}

var expensiveQueryPrefixes = []string{
	"/api/v1/cloud",
	"/api/v1/infrastructure",
	"/api/v1/llm",
	"/api/v1/logs",
	"/api/v1/metrics",
	"/api/v1/saturation",
	"/api/v1/services",
	"/api/v1/traces",
}
