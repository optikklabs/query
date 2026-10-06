package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"
)

func ErrorRecovery() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() { //nolint:contextcheck // WriteJSON logs encode failures without a request; it has no ctx to pass
				recovered := recover()
				if recovered == nil {
					return
				}
				// The server's own signal to abort a response must keep unwinding.
				if err, ok := recovered.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(recovered)
				}
				slog.ErrorContext(r.Context(), "panic recovered",
					slog.Any("error", recovered),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("ip", httputil.ClientIP(r)),
					slog.String("request_id", RequestIDFrom(r.Context())),
					slog.String("stack", string(debug.Stack())),
				)
				httputil.RespondErrorWithCause(w, r, http.StatusInternalServerError, errorcode.Internal, "An unexpected error occurred", nil)
			}()
			next.ServeHTTP(w, r)
		})
	}
}
