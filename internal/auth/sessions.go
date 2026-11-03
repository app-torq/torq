package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/db"
	"github.com/app-torq/torq/internal/store"
)

const (
	// SessionTTL is the sliding inactivity window (FR-004).
	SessionTTL = 30 * 24 * time.Hour
	// touchInterval limits last_seen_at writes to once per hour per session.
	touchInterval = time.Hour
	tokenBytes    = 32
	// maxDeviceName is the longest device name kept, in runes (FR-008).
	maxDeviceName = 64
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

// CleanDeviceName turns a client-supplied device name into a safe label:
// control characters removed, spaces trimmed, at most 64 runes.
func CleanDeviceName(name string) string {
	name = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name))
	if r := []rune(name); len(r) > maxDeviceName {
		name = strings.TrimSpace(string(r[:maxDeviceName]))
	}
	return name
}

// Create starts a session for userID and returns the raw token, which the
// client keeps as a cookie or a bearer token. deviceName is an optional label.
func (s *Sessions) Create(ctx context.Context, userID int64, userAgent, ip, deviceName string) (string, error) {
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
		DeviceName: CleanDeviceName(deviceName),
	})
	if err != nil {
		return "", err
	}
	return raw, nil
}

// Lookup validates raw and returns the session's user and ID, extending the
// sliding expiry and recording the client's ip at most once per hour.
func (s *Sessions) Lookup(ctx context.Context, raw, ip string) (User, int64, error) {
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
			Ip:         ip,
			ID:         row.Session.ID,
		}); err != nil {
			return User{}, 0, err
		}
	}
	return FromStore(row.User), row.Session.ID, nil
}

// SessionInfo is one of a user's sign-ins as shown in the Devices list.
type SessionInfo struct {
	ID         int64
	DeviceName string
	Browser    string
	OS         string
	CreatedAt  string
	LastSeenAt string
	IP         string
}

// List returns the user's unexpired sessions, most recently active first.
func (s *Sessions) List(ctx context.Context, userID int64) ([]SessionInfo, error) {
	rows, err := store.New(s.DB.Reader).ListUserSessions(ctx, store.ListUserSessionsParams{
		UserID: userID, ExpiresAt: clock.Timestamp(s.Clock.Now()),
	})
	if err != nil {
		return nil, err
	}
	out := make([]SessionInfo, len(rows))
	for i, r := range rows {
		browser, os := ParseUserAgent(r.UserAgent)
		out[i] = SessionInfo{
			ID: r.ID, DeviceName: r.DeviceName, Browser: browser, OS: os,
			CreatedAt: r.CreatedAt, LastSeenAt: r.LastSeenAt, IP: r.Ip,
		}
	}
	return out, nil
}

// RevokeOwn ends one of the user's sessions. Another user's session is
// reported as not found, the same as an unknown ID.
func (s *Sessions) RevokeOwn(ctx context.Context, userID, sessionID int64) error {
	n, err := store.New(s.DB.Writer).DeleteUserSession(ctx, store.DeleteUserSessionParams{ID: sessionID, UserID: userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return apperr.NotFound()
	}
	return nil
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
