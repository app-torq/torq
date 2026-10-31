package httpapi

import (
	"errors"
	"net/http"

	"github.com/marekbeckmann/torq/internal/apperr"
	"github.com/marekbeckmann/torq/internal/auth"
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

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	// Secure follows TORQ_COOKIE_SECURE / TORQ_BASE_URL: many self-hosters run plain HTTP on the LAN.
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is configuration-driven, see above
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(auth.SessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   s.Config.SecureCookies(),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is configuration-driven, see setSessionCookie
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.Config.SecureCookies(),
		SameSite: http.SameSiteLaxMode,
	})
}

// currentUser returns the authenticated user; routes guarded by member/admin always have one.
func currentUser(r *http.Request) auth.User {
	u, _ := auth.UserFrom(r.Context())
	return u
}
