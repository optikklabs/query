package token

import (
	"net/http"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/shared/httputil"
)

const RefreshCookiePath = httputil.APIV1Base + "/auth"

var legacyRefreshCookiePaths = []string{"/"}

type cookieOpts struct {
	name     string
	domain   string
	secure   bool
	sameSite http.SameSite
}

func (s *Service) RefreshCookieName() string {
	return s.cookie.name
}

func (s *Service) RefreshCookieValues(r *http.Request) []string {
	var values []string
	for _, c := range r.Cookies() {
		if c.Name == s.cookie.name && c.Value != "" {
			values = append(values, c.Value)
		}
	}
	return values
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

	s.expireCookieAt(w, legacyRefreshCookiePaths...)
}

func (s *Service) ClearRefreshCookie(w http.ResponseWriter) {
	s.expireCookieAt(w, append([]string{RefreshCookiePath}, legacyRefreshCookiePaths...)...)
}

func (s *Service) expireCookieAt(w http.ResponseWriter, paths ...string) {
	for _, path := range paths {
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
