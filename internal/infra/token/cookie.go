package token

import (
	"net/http"
	"strings"
)

// RefreshCookiePath scopes the refresh cookie to the auth endpoints so it
// reaches both /refresh (rotation) and /logout (revocation).
const RefreshCookiePath = "/api/v1/auth"

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
		HttpOnly: true,
		Secure:   s.cookie.secure,
		SameSite: s.cookie.sameSite,
	})
}

func (s *Service) ClearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookie.name,
		Value:    "",
		Path:     RefreshCookiePath,
		Domain:   s.cookie.domain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cookie.secure,
		SameSite: s.cookie.sameSite,
	})
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
