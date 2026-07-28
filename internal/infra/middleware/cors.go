package middleware

import (
	"net/http"
	"strings"
)

func CORSMiddleware(allowedOrigins string) func(http.Handler) http.Handler {
	origins := make([]string, 0, 8)
	for _, o := range strings.Split(allowedOrigins, ",") {
		origin := strings.TrimSpace(o)
		if origin == "" {
			continue
		}
		origins = append(origins, origin)
	}

	match := func(origin string) (allowed, wildcard bool) {
		if origin == "" || len(origins) == 0 {
			return false, false
		}
		for _, a := range origins {
			if a == "*" {
				return true, true
			}
			if a == origin {
				return true, false
			}
			if strings.HasPrefix(a, "*.") && strings.HasSuffix(origin, a[1:]) {
				return true, false
			}
		}
		return false, false
	}

	setOrigin := func(headers http.Header, origin string) bool {
		allowed, wildcard := match(origin)
		if !allowed {
			return false
		}
		if wildcard {
			headers.Set("Access-Control-Allow-Origin", "*")
			return true
		}
		headers.Set("Access-Control-Allow-Origin", origin)
		headers.Set("Access-Control-Allow-Credentials", "true")
		headers.Set("Vary", "Origin")
		return true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			headers := w.Header()
			setOrigin(headers, origin)

			if r.Method == http.MethodOptions {
				headers.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				headers.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-Id, X-User-Id, X-User-Email, X-User-Role, traceparent, tracestate")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			headers.Set("Access-Control-Expose-Headers", "X-Tenant-Id")
			next.ServeHTTP(w, r)
		})
	}
}
