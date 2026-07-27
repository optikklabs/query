package token

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/optikklabs/query/internal/config"
)

func testCookieService() *Service {
	return NewService(config.Config{Auth: config.AuthConfig{
		JWTSecret:         "test-secret",
		RefreshTTLMs:      60_000,
		RefreshCookieName: "optikk_refresh",
		CookieDomain:      "optikk.in",
		CookieSecure:      true,
		CookieSameSite:    "lax",
	}})
}

func TestSetRefreshCookieUsesConfiguredScope(t *testing.T) {
	recorder := httptest.NewRecorder()
	testCookieService().SetRefreshCookie(recorder, "refresh-token")

	cookies := recorder.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cookies = %d, want token plus legacy-path deletion", len(cookies))
	}

	refresh := cookies[0]
	if refresh.Name != "optikk_refresh" || refresh.Value != "refresh-token" {
		t.Fatalf("unexpected refresh cookie: %#v", refresh)
	}
	if refresh.Domain != "optikk.in" || refresh.Path != RefreshCookiePath {
		t.Fatalf("refresh scope = domain %q path %q", refresh.Domain, refresh.Path)
	}
	if !refresh.HttpOnly || !refresh.Secure || refresh.SameSite != http.SameSiteLaxMode {
		t.Fatalf("refresh security attributes are incomplete: %#v", refresh)
	}
	if refresh.MaxAge <= 0 {
		t.Fatalf("refresh MaxAge = %d, want positive", refresh.MaxAge)
	}

	legacyDeletion := cookies[1]
	if legacyDeletion.Domain != "optikk.in" || legacyDeletion.Path != "/" {
		t.Fatalf("legacy deletion scope = domain %q path %q", legacyDeletion.Domain, legacyDeletion.Path)
	}
	if legacyDeletion.MaxAge >= 0 {
		t.Fatalf("legacy deletion MaxAge = %d, want negative", legacyDeletion.MaxAge)
	}
}

func TestRefreshCookieValuesReturnsEveryCandidate(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	request.Header.Add("Cookie", "optikk_refresh=current")
	request.Header.Add("Cookie", "optikk_refresh=legacy")

	values := testCookieService().RefreshCookieValues(request)
	if len(values) != 2 || values[0] != "current" || values[1] != "legacy" {
		t.Fatalf("RefreshCookieValues() = %#v", values)
	}
}
