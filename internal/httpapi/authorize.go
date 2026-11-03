package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/auth"
	"github.com/app-torq/torq/internal/config"
)

const sessionCookie = "torq_session"

// viaBearerKey marks a request whose session came from a bearer token.
type viaBearerKey struct{}

// authorize resolves the session from a bearer token or the session cookie and
// enforces the route's access level. It returns the request with the
// authenticated user in its context.
func (s *Server) authorize(r *http.Request, level access) (*http.Request, error) {
	raw, viaBearer, ok := sessionToken(r)
	if !ok {
		return r, apperr.Unauthenticated()
	}
	user, sessionID, err := s.sessions.Lookup(r.Context(), raw, auth.ClientIP(r, s.Config.TrustedProxies))
	if errors.Is(err, auth.ErrNoSession) {
		return r, apperr.Unauthenticated()
	}
	if err != nil {
		return r, err
	}
	if level == admin && !user.IsAdmin() {
		return r, apperr.Forbidden("auth.forbidden")
	}
	ctx := auth.WithUser(r.Context(), user, sessionID)
	if viaBearer {
		ctx = context.WithValue(ctx, viaBearerKey{}, true)
	}
	return r.WithContext(ctx), nil
}

// sessionToken returns the raw session token and whether it came as a bearer
// token. A bearer token always wins: when one is sent, the cookie is never a
// fallback (research M2). Other Authorization schemes belong to a reverse
// proxy, such as basic auth in front of torq, and are ignored.
func sessionToken(r *http.Request) (raw string, viaBearer, ok bool) {
	if token, isBearer := bearerToken(r); isBearer {
		return token, true, token != ""
	}
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return "", false, false
	}
	return c.Value, false, true
}

// bearerToken returns the token of an "Authorization: Bearer <token>" header
// and whether the header uses the Bearer scheme at all.
func bearerToken(r *http.Request) (string, bool) {
	scheme, token, _ := strings.Cut(strings.TrimSpace(r.Header.Get("Authorization")), " ")
	if !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	return strings.TrimSpace(token), true
}

// viaBearer reports whether the request's session came from a bearer token.
func viaBearer(r *http.Request) bool {
	v, _ := r.Context().Value(viaBearerKey{}).(bool)
	return v
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
