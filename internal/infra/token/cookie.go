package token

import (
	"net/http"
	"strings"
	"time"

	"github.com/optikklabs/query/internal/shared/httputil"
)

// RefreshCookiePath scopes the refresh cookie to the auth endpoints only. The
// token is a sensitive credential, so it is deliberately not sent on every
// request; this path still reaches /refresh and /logout. It must stay stable:
// a cookie is keyed by (name, path), so changing it orphans the old cookie
// instead of replacing it.
const RefreshCookiePath = httputil.APIV1Base + "/auth"

// legacyRefreshCookiePaths are paths older builds wrote the refresh cookie to.
// A path change leaves the previous cookie behind, and browsers send the
// more-specific path first, so a stale sibling can shadow the current token and
// force spurious logouts. We clear these on every write and on logout so the
// duplicate disappears within one refresh cycle.
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

// RefreshCookieValues returns every refresh-cookie value on the request. A
// browser can hold more than one cookie of the same name (e.g. mid cookie-path
// migration); returning all of them lets the caller accept whichever still
// validates instead of trusting only the first the browser happened to send.
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
	// Evict any cookie left at an old path, or it keeps shadowing this one.
	s.expireCookieAt(w, legacyRefreshCookiePaths...)
}

func (s *Service) ClearRefreshCookie(w http.ResponseWriter) {
	s.expireCookieAt(w, append([]string{RefreshCookiePath}, legacyRefreshCookiePaths...)...)
}

// expireCookieAt writes a deletion for the refresh cookie at each path. Each
// (name, path) pair is a distinct cookie to the browser, so clearing must
// target every path the cookie may live at.
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
