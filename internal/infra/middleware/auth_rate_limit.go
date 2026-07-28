package middleware

import (
	"net/http"
	"time"

	types "github.com/optikklabs/query/internal/shared/contracts"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"
)

const (
	maxAuthClients = 50_000
	authClientTTL  = 15 * time.Minute
)

func PublicAuthRateLimit(requestsPerSecond float64, burst int) func(http.Handler) http.Handler {
	limiter := newKeyedLimiter[string](requestsPerSecond, burst, maxAuthClients, authClientTTL)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isPublicRequest(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			if !limiter.allow(httputil.ClientIP(r)) {
				w.Header().Set("Retry-After", "1")
				httputil.WriteJSON(w, http.StatusTooManyRequests, types.Failure(
					errorcode.RateLimited, "Too many authentication requests", r.URL.Path,
				))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
