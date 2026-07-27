package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicAuthRateLimitOnlyAppliesToExactPublicPaths(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := PublicAuthRateLimit(1, 1)(next)

	for _, test := range []struct {
		path string
		want int
	}{
		{path: "/api/v1/auth/login", want: http.StatusNoContent},
		{path: "/api/v1/auth/login", want: http.StatusTooManyRequests},
		{path: "/api/v1/auth/login/admin", want: http.StatusNoContent},
		{path: "/api/v1/auth/change-password", want: http.StatusNoContent},
	} {
		req := httptest.NewRequest(http.MethodPost, test.path, nil)
		req.RemoteAddr = "192.0.2.1:1234"
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)

		if res.Code != test.want {
			t.Fatalf("%s status = %d, want %d", test.path, res.Code, test.want)
		}
	}
}
