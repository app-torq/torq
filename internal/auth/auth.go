// Package auth is the authentication boundary (Constitution IV). Domain code
// depends only on User and UserFrom; concrete providers live in subpackages.
package auth

import (
	"context"

	"github.com/app-torq/torq/internal/store"
)

// Role is a household member's permission level.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// Preferences control display language, formatting and units (FR-052).
// Units are display only: values are always stored and sent in metric units.
type Preferences struct {
	Locale         string
	CurrencySymbol string // empty when not overridden
	Formats
}

// Formats are the format and unit preferences that also have instance-wide
// defaults for new users.
type Formats struct {
	DateFormat       string
	NumberFormat     string
	TimeFormat       string
	CurrencyCode     string
	PowerUnit        string
	DistanceUnit     string
	VolumeUnit       string
	TorqueUnit       string
	PressureUnit     string
	DisplacementUnit string
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
			CurrencySymbol: u.CurrencySymbol.String,
			Formats: Formats{
				DateFormat: u.DateFormat, NumberFormat: u.NumberFormat, TimeFormat: u.TimeFormat,
				CurrencyCode: u.CurrencyCode, PowerUnit: u.PowerUnit, DistanceUnit: u.DistanceUnit,
				VolumeUnit: u.VolumeUnit, TorqueUnit: u.TorqueUnit, PressureUnit: u.PressureUnit,
				DisplacementUnit: u.DisplacementUnit,
			},
		},
	}
}
