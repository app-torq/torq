package local

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"

	"github.com/marekbeckmann/torq/internal/clock"
	"github.com/marekbeckmann/torq/internal/db"
	"github.com/marekbeckmann/torq/internal/store"
)

// ErrInvalidCredentials is returned for unknown users and wrong passwords alike.
var ErrInvalidCredentials = errors.New("invalid credentials")

// Provider authenticates local username/password identities.
type Provider struct {
	DB    *db.DB
	Clock clock.Clock
}

// Kind identifies the provider in user_identities.provider.
func (p *Provider) Kind() string { return "local" }

var (
	dummyOnce sync.Once
	dummyHash string
)

// dummy returns a fixed hash so unknown usernames cost the same as wrong passwords.
func dummy() string {
	dummyOnce.Do(func() {
		h, err := Hash("torq-timing-equalizer")
		if err != nil {
			panic(err)
		}
		dummyHash = h
	})
	return dummyHash
}

// Subject is the identity subject for a local username.
func Subject(username string) string { return strings.ToLower(username) }

// AuthenticatePassword returns the user ID for valid credentials. Hashes with
// outdated parameters are upgraded on success.
func (p *Provider) AuthenticatePassword(ctx context.Context, username, password string) (int64, error) {
	q := store.New(p.DB.Reader)
	row, err := q.GetLocalIdentityByUsername(ctx, Subject(username))
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = Verify(dummy(), password)
		return 0, ErrInvalidCredentials
	}
	if err != nil {
		return 0, err
	}
	ok, err := Verify(row.PasswordHash.String, password)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrInvalidCredentials
	}

	w := store.New(p.DB.Writer)
	if NeedsRehash(row.PasswordHash.String) {
		if h, err := Hash(password); err == nil {
			_ = w.UpdatePasswordHash(ctx, store.UpdatePasswordHashParams{
				PasswordHash: sql.NullString{String: h, Valid: true}, UserID: row.UserID,
			})
		}
	}
	_ = w.TouchLastLogin(ctx, store.TouchLastLoginParams{
		LastLoginAt: sql.NullString{String: clock.Timestamp(p.Clock.Now()), Valid: true}, ID: row.IdentityID,
	})
	return row.UserID, nil
}
