package auth

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/auth/local"
	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/db"
	"github.com/app-torq/torq/internal/store"
)

// MinPasswordLength is the minimum password length (FR-005).
const MinPasswordLength = 10

// MaxUsernameLength is the longest username usernameRe accepts.
const MaxUsernameLength = 32

var (
	usernameRe     = regexp.MustCompile(`^[a-zA-Z0-9._-]{3,32}$`)
	currencyCodeRe = regexp.MustCompile(`^[A-Z]{3}$`)
)

// Users manages household accounts.
type Users struct {
	DB       *db.DB
	Clock    clock.Clock
	Sessions *Sessions
}

// NewUser is the input for setup and admin user creation.
type NewUser struct {
	Username    string
	DisplayName string
	Password    string
	// Role (admin or member) picks a built-in role for older clients;
	// RoleID wins when set. Neither means Member.
	Role   Role
	RoleID int64
	Locale string
}

func validateNewUser(in NewUser) error {
	var v apperr.Validation
	v.Check(!usernameRe.MatchString(in.Username), "username", "validation.username_format")
	validateDisplayName(&v, in.DisplayName)
	code := passwordError(in.Password)
	v.Check(code != "", "password", code)
	v.Check(in.Role != "" && in.Role != RoleAdmin && in.Role != RoleMember, "role", "validation.invalid_choice")
	v.Check(in.Locale != "en" && in.Locale != "de", "locale", "validation.invalid_choice")
	return v.Err()
}

// passwordError returns the validation code for an unacceptable new password,
// or "" when it is fine.
func passwordError(password string) string {
	switch {
	case utf8.RuneCountInString(password) < MinPasswordLength:
		return "validation.password_too_short"
	case len(password) > local.MaxPasswordBytes:
		return "validation.password_too_long"
	}
	return ""
}

func validateDisplayName(v *apperr.Validation, name string) {
	n := utf8.RuneCountInString(strings.TrimSpace(name))
	v.Check(n < 1 || n > 64, "display_name", "validation.length")
}

// SetupRequired reports whether no account exists yet (first boot).
func (u *Users) SetupRequired(ctx context.Context) (bool, error) {
	n, err := store.New(u.DB.Reader).CountUsers(ctx)
	return n == 0, err
}

// Setup creates the initial administrator. It fails once any user exists.
// The non-empty fields of formats become the instance defaults, which the
// administrator starts with like every new user.
func (u *Users) Setup(ctx context.Context, in NewUser, formats Formats) (User, error) {
	in.Role = RoleAdmin
	var created User
	err := u.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		n, err := q.CountUsers(ctx)
		if err != nil {
			return err
		}
		if n > 0 {
			return apperr.Conflict("setup.already_completed", nil)
		}
		if formats != (Formats{}) {
			cur, err := q.GetInstanceSettings(ctx)
			if err != nil {
				return err
			}
			if _, err := SaveDefaults(ctx, q, OverlayFormats(DefaultsFromStore(cur), formats)); err != nil {
				return err
			}
		}
		created, err = u.create(ctx, tx, in)
		return err
	})
	return created, err
}

// OverlayFormats returns base with every non-empty field of f applied.
func OverlayFormats(base, f Formats) Formats {
	for _, p := range []struct {
		dst *string
		src string
	}{
		{&base.DateFormat, f.DateFormat}, {&base.NumberFormat, f.NumberFormat}, {&base.TimeFormat, f.TimeFormat},
		{&base.CurrencyCode, f.CurrencyCode}, {&base.PowerUnit, f.PowerUnit}, {&base.DistanceUnit, f.DistanceUnit},
		{&base.VolumeUnit, f.VolumeUnit}, {&base.TorqueUnit, f.TorqueUnit}, {&base.PressureUnit, f.PressureUnit},
		{&base.DisplacementUnit, f.DisplacementUnit},
	} {
		if p.src != "" {
			*p.dst = p.src
		}
	}
	return base
}

// Create adds a household user (admin only).
func (u *Users) Create(ctx context.Context, in NewUser) (User, error) {
	var created User
	err := u.DB.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		created, err = u.create(ctx, tx, in)
		return err
	})
	return created, err
}

func (u *Users) create(ctx context.Context, tx *sql.Tx, in NewUser) (User, error) {
	in.Username = strings.TrimSpace(in.Username)
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if in.Locale == "" {
		in.Locale = "en"
	}
	if err := validateNewUser(in); err != nil {
		return User{}, err
	}
	q := store.New(tx)
	if _, err := q.GetUserByUsername(ctx, in.Username); err == nil {
		return User{}, apperr.Conflict("user.username_taken", nil)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return User{}, err
	}
	role, err := resolveRole(ctx, q, in.RoleID, in.Role)
	if err != nil {
		return User{}, err
	}
	hash, err := local.Hash(in.Password)
	if err != nil {
		return User{}, err
	}
	settings, err := q.GetInstanceSettings(ctx)
	if err != nil {
		return User{}, err
	}
	f := DefaultsFromStore(settings)
	now := clock.Timestamp(u.Clock.Now())
	row, err := q.CreateUser(ctx, store.CreateUserParams{
		Username: in.Username, DisplayName: in.DisplayName, Role: string(legacyRole(role)),
		RoleID: sql.NullInt64{Int64: role.ID, Valid: true}, Locale: in.Locale,
		DateFormat: f.DateFormat, NumberFormat: f.NumberFormat, TimeFormat: f.TimeFormat, CurrencyCode: f.CurrencyCode,
		PowerUnit: f.PowerUnit, DistanceUnit: f.DistanceUnit, VolumeUnit: f.VolumeUnit, TorqueUnit: f.TorqueUnit,
		PressureUnit: f.PressureUnit, DisplacementUnit: f.DisplacementUnit, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return User{}, err
	}
	if err := q.CreateLocalIdentity(ctx, store.CreateLocalIdentityParams{
		UserID: row.ID, Subject: local.Subject(in.Username),
		PasswordHash: sql.NullString{String: hash, Valid: true}, CreatedAt: now,
	}); err != nil {
		return User{}, err
	}
	return FromStore(row), nil
}

// resolveRole returns the role with id, or the built-in role for legacy
// (admin/member, empty = member). An unknown id is a validation error, and
// so is a legacy value that contradicts the id.
func resolveRole(ctx context.Context, q *store.Queries, id int64, legacy Role) (store.Role, error) {
	if id == 0 {
		key := builtinMember
		if legacy == RoleAdmin {
			key = builtinAdmin
		}
		return q.GetRoleByBuiltinKey(ctx, sql.NullString{String: key, Valid: true})
	}
	var v apperr.Validation
	r, err := q.GetRole(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		v.Add("role_id", "validation.invalid_choice")
		return store.Role{}, v.Err()
	}
	if err != nil {
		return store.Role{}, err
	}
	if legacy != "" && legacy != legacyRole(r) {
		v.Add("role", "validation.invalid_choice")
		return store.Role{}, v.Err()
	}
	return r, nil
}

// Get returns one user.
func (u *Users) Get(ctx context.Context, id int64) (User, error) {
	row, err := store.New(u.DB.Reader).GetUser(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, apperr.NotFound()
	}
	if err != nil {
		return User{}, err
	}
	return FromStore(row), nil
}

// List returns all users.
func (u *Users) List(ctx context.Context) ([]User, error) {
	rows, err := store.New(u.DB.Reader).ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]User, 0, len(rows))
	for _, r := range rows {
		out = append(out, FromStore(r))
	}
	return out, nil
}

// UserPatch is an admin update; nil fields are left unchanged.
type UserPatch struct {
	DisplayName *string
	// Role (admin or member) picks a built-in role for older clients;
	// RoleID wins when both are set and must agree with it.
	Role     *Role
	RoleID   *int64
	Disabled *bool
}

// Update applies an admin patch while keeping at least one enabled admin.
func (u *Users) Update(ctx context.Context, id int64, p UserPatch) (User, error) {
	var v apperr.Validation
	if p.DisplayName != nil {
		validateDisplayName(&v, *p.DisplayName)
	}
	if p.Role != nil {
		v.Check(*p.Role != RoleAdmin && *p.Role != RoleMember, "role", "validation.invalid_choice")
	}
	if err := v.Err(); err != nil {
		return User{}, err
	}
	var updated store.User
	err := u.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		cur, err := q.GetUser(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		now := clock.Timestamp(u.Clock.Now())
		updated = cur
		if p.DisplayName != nil {
			params := preferencesParams(cur)
			params.DisplayName, params.UpdatedAt = strings.TrimSpace(*p.DisplayName), now
			updated, err = q.UpdateUserPreferences(ctx, params)
			if err != nil {
				return err
			}
		}
		if p.Role != nil || p.RoleID != nil {
			var roleID int64
			var legacy Role
			if p.RoleID != nil {
				roleID = *p.RoleID
				if roleID <= 0 {
					roleID = -1 // never a valid id
				}
			}
			if p.Role != nil {
				legacy = *p.Role
			}
			role, err := resolveRole(ctx, q, roleID, legacy)
			if err != nil {
				return err
			}
			if updated, err = q.SetUserRole(ctx, store.SetUserRoleParams{
				Role: string(legacyRole(role)), RoleID: sql.NullInt64{Int64: role.ID, Valid: true}, UpdatedAt: now, ID: id,
			}); err != nil {
				return err
			}
		}
		if p.Disabled != nil {
			disabled := sql.NullString{}
			if *p.Disabled {
				disabled = sql.NullString{String: now, Valid: true}
				if err := q.DeleteUserSessions(ctx, id); err != nil {
					return err
				}
			}
			if updated, err = q.SetUserDisabled(ctx, store.SetUserDisabledParams{DisabledAt: disabled, UpdatedAt: now, ID: id}); err != nil {
				return err
			}
		}
		return ensureAdmin(ctx, q)
	})
	if err != nil {
		return User{}, err
	}
	return FromStore(updated), nil
}

// Delete removes a user. Admins cannot delete themselves or the last admin.
func (u *Users) Delete(ctx context.Context, actorID, id int64) error {
	if actorID == id {
		return apperr.Conflict("user.cannot_delete_self", nil)
	}
	return u.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if _, err := q.GetUser(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		} else if err != nil {
			return err
		}
		if err := q.DeleteUser(ctx, id); err != nil {
			return err
		}
		return ensureAdmin(ctx, q)
	})
}

func ensureAdmin(ctx context.Context, q *store.Queries) error {
	n, err := q.CountEnabledAdmins(ctx)
	if err != nil {
		return err
	}
	if n < 1 {
		return apperr.Conflict("user.last_admin", nil)
	}
	return nil
}

// ResetPassword sets a new password (admin action) and ends all the user's sessions.
func (u *Users) ResetPassword(ctx context.Context, id int64, password string) error {
	if code := passwordError(password); code != "" {
		var v apperr.Validation
		v.Add("new_password", code)
		return v.Err()
	}
	hash, err := local.Hash(password)
	if err != nil {
		return err
	}
	return u.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if _, err := q.GetUser(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		} else if err != nil {
			return err
		}
		if err := q.UpdatePasswordHash(ctx, store.UpdatePasswordHashParams{PasswordHash: sql.NullString{String: hash, Valid: true}, UserID: id}); err != nil {
			return err
		}
		return q.DeleteUserSessions(ctx, id)
	})
}

// ChangeOwnPassword verifies current, stores the new password and ends every
// other session of the user (FR-003).
func (u *Users) ChangeOwnPassword(ctx context.Context, userID, keepSessionID int64, current, next string) error {
	identity, err := store.New(u.DB.Reader).GetLocalIdentityByUser(ctx, userID)
	if err != nil {
		return err
	}
	ok, err := local.Verify(identity.PasswordHash.String, current)
	if err != nil {
		return err
	}
	if !ok {
		var v apperr.Validation
		v.Add("current_password", "auth.invalid_credentials")
		return v.Err()
	}
	if code := passwordError(next); code != "" {
		var v apperr.Validation
		v.Add("new_password", code)
		return v.Err()
	}
	hash, err := local.Hash(next)
	if err != nil {
		return err
	}
	return u.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if err := q.UpdatePasswordHash(ctx, store.UpdatePasswordHashParams{PasswordHash: sql.NullString{String: hash, Valid: true}, UserID: userID}); err != nil {
			return err
		}
		return q.DeleteUserSessionsExcept(ctx, store.DeleteUserSessionsExceptParams{UserID: userID, ID: keepSessionID})
	})
}

// priorityDisplays are how a user can have project priorities shown.
var priorityDisplays = []string{"both", "color", "name"}

// PreferencesPatch updates the caller's own display settings. Formats holds
// the format and unit choices; empty fields stay unchanged.
type PreferencesPatch struct {
	DisplayName     *string
	Locale          *string
	CurrencySymbol  *string // "" clears the override
	PriorityDisplay *string
	CardTagLimit    *int64
	Formats         Formats
}

func preferencesParams(u store.User) store.UpdateUserPreferencesParams {
	return store.UpdateUserPreferencesParams{
		DisplayName: u.DisplayName, Locale: u.Locale, DateFormat: u.DateFormat, NumberFormat: u.NumberFormat,
		TimeFormat: u.TimeFormat, CurrencyCode: u.CurrencyCode, CurrencySymbol: u.CurrencySymbol,
		PowerUnit: u.PowerUnit, DistanceUnit: u.DistanceUnit, VolumeUnit: u.VolumeUnit, TorqueUnit: u.TorqueUnit,
		PressureUnit: u.PressureUnit, DisplacementUnit: u.DisplacementUnit, PriorityDisplay: u.PriorityDisplay,
		CardTagLimit: u.CardTagLimit, ID: u.ID,
	}
}

// UpdatePreferences validates and stores the caller's preferences.
func (u *Users) UpdatePreferences(ctx context.Context, userID int64, p PreferencesPatch) (User, error) {
	cur, err := store.New(u.DB.Reader).GetUser(ctx, userID)
	if err != nil {
		return User{}, err
	}
	next := preferencesParams(cur)
	next.UpdatedAt = clock.Timestamp(u.Clock.Now())
	var v apperr.Validation
	if p.DisplayName != nil {
		validateDisplayName(&v, *p.DisplayName)
		next.DisplayName = strings.TrimSpace(*p.DisplayName)
	}
	if p.Locale != nil {
		v.Check(*p.Locale != "en" && *p.Locale != "de", "locale", "validation.invalid_choice")
		next.Locale = *p.Locale
	}
	if p.CurrencySymbol != nil {
		sym := strings.TrimSpace(*p.CurrencySymbol)
		v.Check(utf8.RuneCountInString(sym) > 5, "currency_symbol", "validation.length")
		next.CurrencySymbol = sql.NullString{String: sym, Valid: sym != ""}
	}
	if p.PriorityDisplay != nil {
		v.Check(!slices.Contains(priorityDisplays, *p.PriorityDisplay), "priority_display", "validation.invalid_choice")
		next.PriorityDisplay = *p.PriorityDisplay
	}
	if p.CardTagLimit != nil {
		v.Check(*p.CardTagLimit < 0 || *p.CardTagLimit > 20, "card_tag_limit", "validation.range")
		next.CardTagLimit = *p.CardTagLimit
	}
	f := OverlayFormats(FromStore(cur).Prefs.Formats, p.Formats)
	f.Validate(&v, "")
	if err := v.Err(); err != nil {
		return User{}, err
	}
	next.DateFormat, next.NumberFormat, next.TimeFormat, next.CurrencyCode = f.DateFormat, f.NumberFormat, f.TimeFormat, f.CurrencyCode
	next.PowerUnit, next.DistanceUnit, next.VolumeUnit = f.PowerUnit, f.DistanceUnit, f.VolumeUnit
	next.TorqueUnit, next.PressureUnit, next.DisplacementUnit = f.TorqueUnit, f.PressureUnit, f.DisplacementUnit
	row, err := store.New(u.DB.Writer).UpdateUserPreferences(ctx, next)
	if err != nil {
		return User{}, err
	}
	return FromStore(row), nil
}
