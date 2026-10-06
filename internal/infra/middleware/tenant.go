package middleware

import (
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/optikklabs/query/internal/infra/metrics"
	"github.com/optikklabs/query/internal/infra/token"
	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"

	types "github.com/optikklabs/query/internal/shared/contracts"
)

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

// deny counts and logs an auth denial, then answers with status/code.
func deny(w http.ResponseWriter, r *http.Request, status int, code, msg string, attrs ...any) {
	metrics.AuthDenied.WithLabelValues(strings.ToLower(code)).Inc()
	attrs = append(attrs,
		slog.String("method", r.Method), slog.String("path", r.URL.Path), slog.String("code", code),
		slog.String("ip", httputil.ClientIP(r)), slog.String("request_id", RequestIDFrom(r.Context())))
	slog.WarnContext(r.Context(), "AUTH_DENIED", attrs...)
	httputil.RespondErrorWithCause(w, r, status, code, msg, nil)
}

func abortUnauthorized(w http.ResponseWriter, r *http.Request) {
	deny(w, r, http.StatusUnauthorized, errorcode.Unauthorized, "Valid authentication is required")
}

// resolveTenant picks the X-Tenant-Id tenant, or the session's default tenant
// when the header is absent.
func resolveTenant(w http.ResponseWriter, r *http.Request, state token.AuthState) (int64, bool) {
	header := r.Header.Get("X-Tenant-Id")
	if header == "" {
		return state.DefaultTenantID, true
	}
	requested, err := strconv.ParseInt(header, 10, 64)
	if err != nil {
		httputil.RespondErrorWithCause(w, r, http.StatusBadRequest, errorcode.Validation, "X-Tenant-Id must be a tenant id", nil)
		return 0, false
	}
	if !slices.Contains(state.TenantIDs, requested) {
		deny(w, r, http.StatusForbidden, "FORBIDDEN_TENANT", "You are not a member of the requested tenant",
			slog.String("user", state.Email), slog.Int64("requested_tenant", requested))
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

// RequireAdmin answers 403 unless the caller is a tenant admin.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if types.TenantFrom(r.Context()).UserRole != "admin" {
			deny(w, r, http.StatusForbidden, errorcode.Forbidden, "Only tenant admins can manage this resource")
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

			ctx := types.WithTenant(r.Context(), types.TenantContext{
				TenantID:  tenantID,
				UserID:    authState.UserID,
				UserEmail: authState.Email,
				UserRole:  authState.Role,
			})
			metrics.AuthAuthenticated.Inc()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
