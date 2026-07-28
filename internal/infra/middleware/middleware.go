package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/optikklabs/query/internal/infra/metrics"
	"github.com/optikklabs/query/internal/infra/token"
	"github.com/optikklabs/query/internal/shared/errorcode"

	"github.com/optikklabs/query/internal/infra/utils"
	types "github.com/optikklabs/query/internal/shared/contracts"
	"github.com/optikklabs/query/internal/shared/httputil"
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

func ErrorRecovery() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}
				if recovered == http.ErrAbortHandler {
					panic(recovered)
				}
				slog.Error("panic recovered",
					slog.Any("error", recovered),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("ip", httputil.ClientIP(r)),
					slog.String("request_id", RequestIDFrom(r.Context())),
					slog.String("stack", string(debug.Stack())),
				)
				httputil.WriteJSON(w, http.StatusInternalServerError,
					types.Failure(errorcode.Internal, "An unexpected error occurred", r.URL.Path))
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func BodyLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				next.ServeHTTP(w, r)
				return
			}
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

var publicPaths = map[string]struct{}{
	httputil.APIV1Base + "/auth/signup":          {},
	httputil.APIV1Base + "/auth/login":           {},
	httputil.APIV1Base + "/auth/refresh":         {},
	httputil.APIV1Base + "/auth/logout":          {},
	httputil.APIV1Base + "/auth/device/code":     {},
	httputil.APIV1Base + "/auth/device/token":    {},
	httputil.APIV1Base + "/auth/verify-email":    {},
	httputil.APIV1Base + "/auth/forgot-password": {},
	httputil.APIV1Base + "/auth/reset-password":  {},
}

func isPublicRequest(path string) bool {
	_, ok := publicPaths[path]
	return ok
}

func abortUnauthorized(w http.ResponseWriter, r *http.Request) {
	metrics.AuthDenied.WithLabelValues("unauthorized").Inc()
	slog.WarnContext(r.Context(), "AUTH_DENIED", slog.String("method", r.Method), slog.String("path", r.URL.Path), slog.String("code", "UNAUTHORIZED"), slog.String("ip", httputil.ClientIP(r)), slog.String("request_id", RequestIDFrom(r.Context())))
	httputil.WriteJSON(w, http.StatusUnauthorized, types.Failure(
		errorcode.Unauthorized, "Valid authentication is required", r.URL.Path,
	))
}

func abortMissingTenant(w http.ResponseWriter, r *http.Request, email string) {
	metrics.AuthDenied.WithLabelValues("missing_tenant").Inc()
	slog.WarnContext(r.Context(), "AUTH_DENIED", slog.String("method", r.Method), slog.String("path", r.URL.Path), slog.String("code", "MISSING_TENANT"), slog.String("user", email), slog.String("ip", httputil.ClientIP(r)), slog.String("request_id", RequestIDFrom(r.Context())))
	httputil.WriteJSON(w, http.StatusForbidden, types.Failure(
		"MISSING_TENANT", "Session does not contain a valid tenant_id", r.URL.Path,
	))
}

func abortForbiddenTenant(w http.ResponseWriter, r *http.Request, email string, requestedTenantID int64) {
	metrics.AuthDenied.WithLabelValues("forbidden_tenant").Inc()
	slog.WarnContext(r.Context(), "AUTH_DENIED", slog.String("method", r.Method), slog.String("path", r.URL.Path), slog.String("code", "FORBIDDEN_TENANT"), slog.String("user", email), slog.Int64("requested_tenant", requestedTenantID), slog.String("ip", httputil.ClientIP(r)), slog.String("request_id", RequestIDFrom(r.Context())))
	httputil.WriteJSON(w, http.StatusForbidden, types.Failure(
		"FORBIDDEN_TENANT", "You are not a member of the requested tenant", r.URL.Path,
	))
}

func resolveTenant(w http.ResponseWriter, r *http.Request, state token.AuthState) (int64, bool) {
	requested := utils.ToInt64(r.Header.Get("X-Tenant-Id"), 0)
	if requested == 0 {
		if state.DefaultTenantID == 0 {
			abortMissingTenant(w, r, state.Email)
			return 0, false
		}
		return state.DefaultTenantID, true
	}
	if !authorizedForTenant(state.TenantIDs, state.DefaultTenantID, requested) {
		abortForbiddenTenant(w, r, state.Email, requested)
		return 0, false
	}
	return requested, true
}

func bearerAuthState(r *http.Request, tokens *token.Service) (token.AuthState, bool) {
	header := r.Header.Get("Authorization")
	raw, found := strings.CutPrefix(header, "Bearer ")
	if !found || raw == "" {
		return token.AuthState{}, false
	}
	state, err := tokens.ParseAccess(raw)
	if err != nil {
		return token.AuthState{}, false
	}
	return state, true
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if types.TenantFrom(r.Context()).UserRole != "admin" {
			abortUnauthorized(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func TenantMiddleware(tokens *token.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authState, ok := bearerAuthState(r, tokens)
			if !ok {
				if isPublicRequest(r.URL.Path) {
					next.ServeHTTP(w, r)
					return
				}
				abortUnauthorized(w, r)
				return
			}

			tenantID, ok := resolveTenant(w, r, authState)
			if !ok {
				return
			}

			role := authState.Role
			if role == "" {
				role = "member"
			}

			ctx := types.WithTenant(r.Context(), types.TenantContext{
				TenantID:  tenantID,
				UserID:    authState.UserID,
				UserEmail: authState.Email,
				UserRole:  role,
			})
			metrics.AuthAuthenticated.Inc()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func authorizedForTenant(tenantIDs []int64, defaultTenantID, requestedTenantID int64) bool {
	if len(tenantIDs) == 0 {
		return defaultTenantID == requestedTenantID
	}
	for _, tenantID := range tenantIDs {
		if tenantID == requestedTenantID {
			return true
		}
	}
	return false
}
