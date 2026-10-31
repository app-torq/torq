// Package auth is the authentication boundary (Constitution IV). Domain code
// depends only on User and UserFrom; concrete providers live in subpackages.
package auth

import (
	"context"

	"github.com/marekbeckmann/torq/internal/store"
)

// Role is a household member's permission level.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// Preferences control display language and formatting (FR-052).
type Preferences struct {
	Locale         string
	DateFormat     string
	NumberFormat   string
	CurrencyCode   string
	CurrencySymbol string // empty when not overridden
	PowerUnit      string // "kw" or "ps"; display only, power is always stored in kW
}

// User is the authenticated, provider-agnostic user.
type User struct {
	ID          int64
	Username    string
	DisplayName string
	Role        Role
	Disabled    bool
	Prefs       Preferences
}

// IsAdmin reports whether the user may manage users, webhooks and settings.
func (u User) IsAdmin() bool { return u.Role == RoleAdmin }

// Provider is implemented by every authentication mechanism.
type Provider interface {
	Kind() string
}

// PasswordAuthenticator is a provider that verifies username/password credentials.
type PasswordAuthenticator interface {
	Provider
	AuthenticatePassword(ctx context.Context, username, password string) (userID int64, err error)
}

type ctxKey struct{}

type ctxValue struct {
	user      User
	sessionID int64
}

// WithUser returns a context carrying the authenticated user and session.
func WithUser(ctx context.Context, u User, sessionID int64) context.Context {
	return context.WithValue(ctx, ctxKey{}, ctxValue{user: u, sessionID: sessionID})
}

// UserFrom returns the authenticated user, if any.
func UserFrom(ctx context.Context) (User, bool) {
	v, ok := ctx.Value(ctxKey{}).(ctxValue)
	return v.user, ok
}

// SessionIDFrom returns the current session ID, if any.
func SessionIDFrom(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(ctxKey{}).(ctxValue)
	return v.sessionID, ok
}

// FromStore converts a database row into a User.
func FromStore(u store.User) User {
	return User{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Role:        Role(u.Role),
		Disabled:    u.DisabledAt.Valid,
		Prefs: Preferences{
			Locale:         u.Locale,
			DateFormat:     u.DateFormat,
			NumberFormat:   u.NumberFormat,
			CurrencyCode:   u.CurrencyCode,
			CurrencySymbol: u.CurrencySymbol.String,
			PowerUnit:      u.PowerUnit,
		},
	}
}
