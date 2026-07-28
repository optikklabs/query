package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	types "github.com/optikklabs/query/internal/shared/contracts"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"
)

const (
	maxAuthClients = 50_000
	authClientTTL  = 15 * time.Minute
)

type authClientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type authRateLimiter struct {
	mu      sync.Mutex
	clients map[string]*authClientLimiter
	limit   rate.Limit
	burst   int
}

func newAuthRateLimiter(requestsPerSecond float64, burst int) *authRateLimiter {
	return &authRateLimiter{
		clients: make(map[string]*authClientLimiter),
		limit:   rate.Limit(requestsPerSecond),
		burst:   burst,
	}
}

func (l *authRateLimiter) allow(clientIP string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	client := l.clients[clientIP]
	if client == nil {
		if len(l.clients) >= maxAuthClients {
			l.evictIdleOrOldest(now)
		}
		client = &authClientLimiter{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.clients[clientIP] = client
	}
	client.lastSeen = now
	return client.limiter.AllowN(now, 1)
}

func (l *authRateLimiter) evictIdleOrOldest(now time.Time) {
	var oldestIP string
	var oldest time.Time
	for ip, client := range l.clients {
		if now.Sub(client.lastSeen) > authClientTTL {
			delete(l.clients, ip)
			continue
		}
		if oldestIP == "" || client.lastSeen.Before(oldest) {
			oldestIP = ip
			oldest = client.lastSeen
		}
	}
	if len(l.clients) >= maxAuthClients && oldestIP != "" {
		delete(l.clients, oldestIP)
	}
}

func PublicAuthRateLimit(requestsPerSecond float64, burst int) func(http.Handler) http.Handler {
	limiter := newAuthRateLimiter(requestsPerSecond, burst)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isPublicRequest(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			if !limiter.allow(httputil.ClientIP(r), time.Now()) {
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
