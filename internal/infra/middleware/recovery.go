package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/optikklabs/query/internal/shared/errorcode"
	"github.com/optikklabs/query/internal/shared/httputil"

	types "github.com/optikklabs/query/internal/shared/contracts"
)

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
