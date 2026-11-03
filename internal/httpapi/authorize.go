package httpapi

import (
	"errors"
	"net/http"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/auth"
	"github.com/app-torq/torq/internal/config"
)

const sessionCookie = "torq_session"

// authorize resolves the session cookie and enforces the route's access level.
// It returns the request with the authenticated user in its context.
func (s *Server) authorize(r *http.Request, level access) (*http.Request, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return r, apperr.Unauthenticated()
	}
	user, sessionID, err := s.sessions.Lookup(r.Context(), c.Value)
	if errors.Is(err, auth.ErrNoSession) {
		return r, apperr.Unauthenticated()
	}
	if err != nil {
		return r, err
	}
	if level == admin && !user.IsAdmin() {
		return r, apperr.Forbidden("auth.forbidden")
	}
	return r.WithContext(auth.WithUser(r.Context(), user, sessionID)), nil
}

// secureCookies reports whether the session cookie gets the Secure flag. It
// follows TORQ_COOKIE_SECURE / TORQ_BASE_URL, since many self-hosters run plain
// HTTP on the LAN. In auto mode a request that arrived over HTTPS also counts,
// so a missing TORQ_BASE_URL behind a TLS proxy doesn't leave the flag off.
func (s *Server) secureCookies(r *http.Request) bool {
	if s.Config.SecureCookies() {
		return true
	}
	return s.Config.CookieSecure == config.CookieSecureAuto && auth.IsHTTPS(r, s.Config.TrustedProxies)
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is configuration-driven, see secureCookies
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(auth.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   s.secureCookies(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is configuration-driven, see secureCookies
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secureCookies(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// currentUser returns the authenticated user; routes guarded by member/admin always have one.
func currentUser(r *http.Request) auth.User {
	u, _ := auth.UserFrom(r.Context())
	return u
}
