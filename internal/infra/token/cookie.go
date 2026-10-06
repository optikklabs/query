package token

import (
	"net/http"
	"time"

	"github.com/optikklabs/query/internal/shared/httputil"
)

const RefreshCookiePath = httputil.APIV1Base + "/auth"

type cookieOpts struct {
	name     string
	domain   string
	secure   bool
	sameSite http.SameSite
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
}

func (s *Service) ClearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cookie.name,
		Value:    "",
		Path:     RefreshCookiePath,
		Domain:   s.cookie.domain,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   s.cookie.secure,
		SameSite: s.cookie.sameSite,
	})
}

// sameSiteModes maps the validated config.CookieSameSiteModes values.
var sameSiteModes = map[string]http.SameSite{
	"lax":    http.SameSiteLaxMode,
	"strict": http.SameSiteStrictMode,
	"none":   http.SameSiteNoneMode,
}
