package middleware

import (
	"log/slog"
	"net/http"
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
	requested, _ := strconv.ParseInt(r.Header.Get("X-Tenant-Id"), 10, 64)
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
