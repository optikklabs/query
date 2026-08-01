package httputil

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestParseIDParam(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  int64
		ok    bool
	}{
		{"42", 42, true}, {"", 0, false}, {"0", 0, false}, {"-1", 0, false},
		{"abc", 0, false}, {"9223372036854775808", 0, false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			route := chi.NewRouteContext()
			route.URLParams.Add("id", tc.value)
			req := httptest.NewRequest("GET", "/", nil)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
			got, ok := ParseIDParam(httptest.NewRecorder(), req, "id")
			if got != tc.want || ok != tc.ok {
				t.Fatalf("ParseIDParam(%q) = (%d, %v), want (%d, %v)", tc.value, got, ok, tc.want, tc.ok)
			}
		})
	}
}
