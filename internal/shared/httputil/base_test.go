package httputil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func TestClientIPTrustsOnlyTraefikXFFEntry(t *testing.T) {
	router := chi.NewRouter()
	router.Use(middleware.ClientIPFromXFFTrustedProxies(1))
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, ClientIP(r))
	})

	for _, tc := range []struct {
		name       string
		xff        string
		remoteAddr string
		want       string
	}{
		{name: "proxy address", xff: "203.0.113.9", remoteAddr: "10.0.0.2:1234", want: "203.0.113.9"},
		{name: "spoofed prefix", xff: "192.0.2.1, 203.0.113.9", remoteAddr: "10.0.0.2:1234", want: "203.0.113.9"},
		{name: "direct fallback", remoteAddr: "198.51.100.4:1234", want: "198.51.100.4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remoteAddr
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if got := resp.Body.String(); got != tc.want {
				t.Fatalf("ClientIP() = %q, want %q", got, tc.want)
			}
		})
	}
}

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

func TestParseRangeRequiresExplicitBounds(t *testing.T) {
	for _, target := range []string{
		"/",
		"/?startTime=1000",
		"/?endTime=2000",
	} {
		req := httptest.NewRequest("GET", target, nil)
		if _, _, err := ParseRange(req); err == nil {
			t.Fatalf("ParseRange(%q) unexpectedly accepted an implicit bound", target)
		}
	}
}

func TestParseRangeAcceptsExplicitAliases(t *testing.T) {
	for _, target := range []string{
		"/?startTime=1000&endTime=2000",
		"/?start=1000&end=2000",
	} {
		req := httptest.NewRequest("GET", target, nil)
		start, end, err := ParseRange(req)
		if err != nil {
			t.Fatalf("ParseRange(%q): %v", target, err)
		}
		if start != 1000 || end != 2000 {
			t.Fatalf("ParseRange(%q) = (%d, %d), want (1000, 2000)", target, start, end)
		}
	}
}
