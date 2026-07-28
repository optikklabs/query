package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

type ctxKey int

const requestIDKey ctxKey = iota

func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-Id")
			if id == "" {
				id = traceIDFromTraceparent(r.Header.Get("traceparent"))
			}
			if id == "" {
				id = newRequestID()
			}
			w.Header().Set("X-Request-Id", id)
			ctx := context.WithValue(r.Context(), requestIDKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func traceIDFromTraceparent(tp string) string {
	parts := strings.Split(tp, "-")
	if len(parts) < 2 || len(parts[1]) != 32 {
		return ""
	}
	return parts[1]
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b[:])
}
