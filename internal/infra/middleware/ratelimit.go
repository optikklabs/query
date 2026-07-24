package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/optikklabs/query/internal/infra/metrics"
	types "github.com/optikklabs/query/internal/shared/contracts"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"
)

type tenantRateLimiter struct {
	mu       sync.Mutex
	limiters map[int64]*tenantLimiter
}

type tenantLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newTenantRateLimiter() *tenantRateLimiter {
	trl := &tenantRateLimiter{
		limiters: make(map[int64]*tenantLimiter),
	}
	go trl.cleanup()
	return trl
}

func (trl *tenantRateLimiter) getLimiter(tenantID int64, reqLimit rate.Limit, burst int) *rate.Limiter {
	trl.mu.Lock()
	defer trl.mu.Unlock()

	tl, exists := trl.limiters[tenantID]
	if !exists {
		tl = &tenantLimiter{
			limiter: rate.NewLimiter(reqLimit, burst),
		}
		trl.limiters[tenantID] = tl
	}
	tl.lastSeen = time.Now()
	return tl.limiter
}

func (trl *tenantRateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute * 5)
	for range ticker.C {
		now := time.Now()
		trl.mu.Lock()
		for tenantID, tl := range trl.limiters {
			if now.Sub(tl.lastSeen) > time.Minute*15 {
				delete(trl.limiters, tenantID)
			}
		}
		trl.mu.Unlock()
	}
}

// TenantRateLimit applies a per-tenant rate limit to HTTP requests.
func TenantRateLimit(reqsPerSec float64, burst int) func(http.Handler) http.Handler {
	rl := newTenantRateLimiter()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := types.TenantFrom(r.Context()).TenantID
			if tenantID == 0 {
				next.ServeHTTP(w, r)
				return
			}

			limiter := rl.getLimiter(tenantID, rate.Limit(reqsPerSec), burst)
			if !limiter.Allow() {
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
