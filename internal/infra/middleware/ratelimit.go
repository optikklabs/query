package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/optikklabs/query/internal/infra/metrics"
	types "github.com/optikklabs/query/internal/shared/contracts"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"
)

const (
	maxTenantLimiterEntries = 10_000
	tenantLimiterTTL        = 15 * time.Minute
)

func TenantRateLimit(reqsPerSec float64, burst int) func(http.Handler) http.Handler {
	rl := newKeyedLimiter[int64](reqsPerSec, burst, maxTenantLimiterEntries, tenantLimiterTTL)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := types.TenantFrom(r.Context()).TenantID
			if tenantID == 0 {
				next.ServeHTTP(w, r)
				return
			}

			if !rl.allow(tenantID) {
				metrics.APIRateLimitedTotal.WithLabelValues(strconv.FormatInt(tenantID, 10)).Inc()
				httputil.WriteJSON(w, http.StatusTooManyRequests, types.Failure(
					errorcode.RateLimited, "Too many requests", r.URL.Path,
				))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
