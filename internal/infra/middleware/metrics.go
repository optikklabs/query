package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/optikklabs/query/internal/infra/metrics"
)

// HTTPMetricsMiddleware populates `optikk_http_*` Prometheus metrics.
// Route labels use chi's RoutePattern() template to bound cardinality.
func HTTPMetricsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			metrics.HTTPInFlight.Inc()
			defer metrics.HTTPInFlight.Dec()

			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "__unmatched__"
			}
			metrics.HTTPRequestsTotal.
				WithLabelValues(route, r.Method, statusClass(ww.Status())).Inc()
			metrics.HTTPDuration.
				WithLabelValues(route, r.Method).
				Observe(time.Since(start).Seconds())
		})
	}
}

func statusClass(status int) string {
	switch {
	case status >= 500:
		return "5xx"
	case status >= 400:
		return "4xx"
	case status >= 300:
		return "3xx"
	case status >= 200:
		return "2xx"
	}
	return strconv.Itoa(status)
}
