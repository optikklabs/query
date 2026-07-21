package token

import (
	"net/http"
	"strings"
	"time"
)

// RefreshCookiePath scopes the refresh cookie to all endpoints.
// Path="/" ensures proxies and different browser contexts do not drop the cookie.
const RefreshCookiePath = "/"

// legacyRefreshCookiePath is the pre-"/" scope. Sessions created before the
// path widened still hold a cookie here; logout must clear it too, or the
// browser keeps sending a stale token that shadows the current one.
const legacyRefreshCookiePath = "/api/v1/auth"

type cookieOpts struct {
	name     string
	domain   string
	secure   bool
	sameSite http.SameSite
}

func (s *Service) RefreshCookieName() string {
	return s.cookie.name
}

func (s *Service) SetRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookie.name,
		Value:    token,
		Path:     RefreshCookiePath,
		Domain:   s.cookie.domain,
		MaxAge:   int(s.refreshTTL.Seconds()),
		Expires:  time.Now().Add(s.refreshTTL),
		HttpOnly: true,
		Secure:   s.cookie.secure,
		SameSite: s.cookie.sameSite,
	})
}

func (s *Service) ClearRefreshCookie(w http.ResponseWriter) {
	// Clear both the current and legacy paths: a browser may hold a cookie at
	// either, and each (name, path) pair is a distinct cookie to the browser.
	for _, path := range []string{RefreshCookiePath, legacyRefreshCookiePath} {
		http.SetCookie(w, &http.Cookie{
			Name:     s.cookie.name,
			Value:    "",
			Path:     path,
			Domain:   s.cookie.domain,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
			HttpOnly: true,
			Secure:   s.cookie.secure,
			SameSite: s.cookie.sameSite,
		})
	}
}

func parseSameSite(raw string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "default":
		return http.SameSiteDefaultMode
	default:
		return http.SameSiteLaxMode
	}
}
