package httputil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/optikklabs/query/internal/shared/filterutil"
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
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
			got, ok := ParseIDParam(httptest.NewRecorder(), req, "id")
			if got != tc.want || ok != tc.ok {
				t.Fatalf("ParseIDParam(%q) = (%d, %v), want (%d, %v)", tc.value, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestParseRangeRejectsInvalidWindows(t *testing.T) {
	for _, target := range []string{
		"/",
		"/?startTime=1000",
		"/?endTime=2000",
		"/?start=1000&end=2000",
		"/?startTime=2000&endTime=1000",
		"/?startTime=1000&endTime=1000",
		"/?startTime=1&endTime=2592000002",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		if _, _, err := parseRange(req, filterutil.MaxTimeRangeMs); err == nil {
			t.Fatalf("parseRange(%q) unexpectedly accepted the window", target)
		}
	}
}

func TestParseRangeAcceptsValidWindow(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?startTime=1000&endTime=2000", nil)
	start, end, err := parseRange(req, filterutil.MaxTimeRangeMs)
	if err != nil {
		t.Fatalf("parseRange: %v", err)
	}
	if start != 1000 || end != 2000 {
		t.Fatalf("parseRange = (%d, %d), want (1000, 2000)", start, end)
	}
}

func TestBindJSON(t *testing.T) {
	type body struct {
		Name string `json:"name"`
	}
	for _, tc := range []struct {
		name, body string
		ok         bool
	}{
		{"valid", `{"name":"a"}`, true},
		{"unknown field", `{"name":"a","extra":1}`, false},
		{"trailing value", `{"name":"a"}{"name":"b"}`, false},
		{"trailing garbage", `{"name":"a"} x`, false},
		{"empty", ``, false},
		{"too large", `{"name":"` + strings.Repeat("a", maxBodyBytes) + `"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			var v body
			if got := BindJSON(rec, req, &v); got != tc.ok {
				t.Fatalf("BindJSON = %v, want %v", got, tc.ok)
			}
			if !tc.ok && rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestParseComparisonRange(t *testing.T) {
	const start, end = int64(10_000_000_000), int64(10_003_600_000)
	for _, tc := range []struct {
		query          string
		wantStart, end int64
		ok             bool
	}{
		{"compareTo=previous_period", start - 3_600_000, end - 3_600_000, true},
		{"compareTo=previous_day", start - 86_400_000, end - 86_400_000, true},
		{"compareStart=1&compareEnd=2", 1, 2, true},
		{"compareTo=bogus", 0, 0, false},
		{"", 0, 0, false},
	} {
		req := httptest.NewRequest(http.MethodGet, "/?"+tc.query, nil)
		s, e, ok := ParseComparisonRange(req, start, end)
		if s != tc.wantStart || e != tc.end || ok != tc.ok {
			t.Errorf("%q = (%d, %d, %v), want (%d, %d, %v)", tc.query, s, e, ok, tc.wantStart, tc.end, tc.ok)
		}
	}
}
