package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/infra/metrics"
)

func ExpensiveQueryLimit(max int) func(http.Handler) http.Handler {
	if max <= 0 {
		max = 1
	}
	semaphores := map[string]chan struct{}{
		workloadDetail:   make(chan struct{}, max),
		workloadExplorer: make(chan struct{}, max),
		workloadOverview: make(chan struct{}, max),
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			workload := queryWorkload(r)
			if workload == "" {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			select {
			case semaphores[workload] <- struct{}{}:
				metrics.QueryWaitDuration.WithLabelValues(workload).Observe(time.Since(start).Seconds())
				metrics.QueryInFlight.WithLabelValues(workload).Inc()
				defer func() {
					metrics.QueryInFlight.WithLabelValues(workload).Dec()
					<-semaphores[workload]
				}()
			case <-r.Context().Done():
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

const (
	workloadDetail   = "detail"
	workloadExplorer = "explorer"
	workloadOverview = "overview"
)

func queryWorkload(r *http.Request) string {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		return ""
	}
	for _, prefix := range expensiveQueryPrefixes {
		if strings.HasPrefix(r.URL.Path, prefix) {
			if isDetailPath(r.URL.Path) {
				return workloadDetail
			}
			if isExplorerPath(r.URL.Path) {
				return workloadExplorer
			}
			return workloadOverview
		}
	}
	return ""
}

func isDetailPath(path string) bool {
	if strings.HasPrefix(path, "/api/v1/traces/") {
		return !hasRouteSegment(path, "/api/v1/traces/", "query", "facets", "trend", "suggest")
	}
	if strings.HasPrefix(path, "/api/v1/logs/") &&
		!hasRouteSegment(path, "/api/v1/logs/", "query", "facets", "suggest") {
		return true
	}
	return false
}

func hasRouteSegment(path, prefix string, segments ...string) bool {
	tail := strings.TrimPrefix(path, prefix)
	for _, segment := range segments {
		if tail == segment || strings.HasPrefix(tail, segment+"/") {
			return true
		}
	}
	return false
}

func isExplorerPath(path string) bool {
	for _, prefix := range []string{
		"/api/v1/logs",
		"/api/v1/metrics",
		"/api/v1/saturation",
		"/api/v1/traces",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

var expensiveQueryPrefixes = []string{
	"/api/v1/infrastructure",
	"/api/v1/llm",
	"/api/v1/logs",
	"/api/v1/metrics",
	"/api/v1/saturation",
	"/api/v1/services",
	"/api/v1/traces",
}
