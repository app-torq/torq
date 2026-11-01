package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"

	"github.com/marekbeckmann/torq/internal/clock"
	"github.com/marekbeckmann/torq/internal/db"
	"github.com/marekbeckmann/torq/internal/store"
)

const (
	// SessionTTL is the sliding inactivity window (FR-004).
	SessionTTL = 30 * 24 * time.Hour
	// touchInterval limits last_seen_at writes to once per hour per session.
	touchInterval = time.Hour
	tokenBytes    = 32
)

// ErrNoSession is returned when a token is unknown, expired or its user disabled.
var ErrNoSession = errors.New("no valid session")

// Sessions issues and validates opaque session tokens. Only SHA-256 digests of
// tokens are stored (research R6).
type Sessions struct {
	DB    *db.DB
	Clock clock.Clock
}

func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

// Create starts a session for userID and returns the raw token for the cookie.
func (s *Sessions) Create(ctx context.Context, userID int64, userAgent, ip string) (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	raw := base64.RawURLEncoding.EncodeToString(b)
	now := s.Clock.Now()
	if len(userAgent) > 256 {
		userAgent = userAgent[:256]
	}
	_, err := store.New(s.DB.Writer).CreateSession(ctx, store.CreateSessionParams{
		UserID:     userID,
		TokenHash:  hashToken(raw),
		CreatedAt:  clock.Timestamp(now),
		LastSeenAt: clock.Timestamp(now),
		ExpiresAt:  clock.Timestamp(now.Add(SessionTTL)),
		UserAgent:  userAgent,
		Ip:         ip,
	})
	if err != nil {
		return "", err
	}
	return raw, nil
}

// Lookup validates raw and returns the session's user and ID, extending the
// sliding expiry at most once per hour.
func (s *Sessions) Lookup(ctx context.Context, raw string) (User, int64, error) {
	if raw == "" {
		return User{}, 0, ErrNoSession
	}
	row, err := store.New(s.DB.Reader).GetSessionByTokenHash(ctx, hashToken(raw))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, 0, ErrNoSession
	}
	if err != nil {
		return User{}, 0, err
	}
	now := s.Clock.Now()
	expires, err := time.Parse(time.RFC3339, row.Session.ExpiresAt)
	if err != nil || !now.Before(expires) {
		return User{}, 0, ErrNoSession
	}
	if last, err := time.Parse(time.RFC3339, row.Session.LastSeenAt); err != nil || now.Sub(last) >= touchInterval {
		if err := store.New(s.DB.Writer).TouchSession(ctx, store.TouchSessionParams{
			LastSeenAt: clock.Timestamp(now),
			ExpiresAt:  clock.Timestamp(now.Add(SessionTTL)),
			ID:         row.Session.ID,
		}); err != nil {
			return User{}, 0, err
		}
	}
	return FromStore(row.User), row.Session.ID, nil
}

// Revoke ends one session.
func (s *Sessions) Revoke(ctx context.Context, sessionID int64) error {
	return store.New(s.DB.Writer).DeleteSession(ctx, sessionID)
}

// RevokeAllExcept ends every session of userID except keepID.
func (s *Sessions) RevokeAllExcept(ctx context.Context, userID, keepID int64) error {
	return store.New(s.DB.Writer).DeleteUserSessionsExcept(ctx, store.DeleteUserSessionsExceptParams{UserID: userID, ID: keepID})
}

// PruneExpired deletes expired sessions and returns how many were removed.
func (s *Sessions) PruneExpired(ctx context.Context) (int64, error) {
	return store.New(s.DB.Writer).DeleteExpiredSessions(ctx, clock.Timestamp(s.Clock.Now()))
}
