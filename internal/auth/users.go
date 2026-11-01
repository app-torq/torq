package auth

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/marekbeckmann/torq/internal/apperr"
	"github.com/marekbeckmann/torq/internal/auth/local"
	"github.com/marekbeckmann/torq/internal/clock"
	"github.com/marekbeckmann/torq/internal/db"
	"github.com/marekbeckmann/torq/internal/store"
)

// MinPasswordLength is the minimum password length (FR-005).
const MinPasswordLength = 10

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
	Role        Role
	Locale      string
}

func validateNewUser(in NewUser) error {
	var v apperr.Validation
	v.Check(!usernameRe.MatchString(in.Username), "username", "validation.username_format")
	validateDisplayName(&v, in.DisplayName)
	v.Check(utf8.RuneCountInString(in.Password) < MinPasswordLength, "password", "validation.password_too_short")
	v.Check(in.Role != RoleAdmin && in.Role != RoleMember, "role", "validation.invalid_choice")
	v.Check(in.Locale != "en" && in.Locale != "de", "locale", "validation.invalid_choice")
	return v.Err()
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
func (u *Users) Setup(ctx context.Context, in NewUser) (User, error) {
	in.Role = RoleAdmin
	var created User
	err := u.DB.WithTx(ctx, func(tx *sql.Tx) error {
		n, err := store.New(tx).CountUsers(ctx)
		if err != nil {
			return err
		}
		if n > 0 {
			return apperr.Conflict("setup.already_completed", nil)
		}
		created, err = u.create(ctx, tx, in)
		return err
	})
	return created, err
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
	hash, err := local.Hash(in.Password)
	if err != nil {
		return User{}, err
	}
	now := clock.Timestamp(u.Clock.Now())
	row, err := q.CreateUser(ctx, store.CreateUserParams{
		Username: in.Username, DisplayName: in.DisplayName, Role: string(in.Role),
		Locale: in.Locale, CreatedAt: now, UpdatedAt: now,
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
	Role        *Role
	Disabled    *bool
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
			updated, err = q.UpdateUserPreferences(ctx, store.UpdateUserPreferencesParams{
				DisplayName: strings.TrimSpace(*p.DisplayName), Locale: cur.Locale, DateFormat: cur.DateFormat,
				NumberFormat: cur.NumberFormat, CurrencyCode: cur.CurrencyCode, CurrencySymbol: cur.CurrencySymbol,
				PowerUnit: cur.PowerUnit, UpdatedAt: now, ID: id,
			})
			if err != nil {
				return err
			}
		}
		if p.Role != nil {
			if updated, err = q.SetUserRole(ctx, store.SetUserRoleParams{Role: string(*p.Role), UpdatedAt: now, ID: id}); err != nil {
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
	if utf8.RuneCountInString(password) < MinPasswordLength {
		var v apperr.Validation
		v.Add("new_password", "validation.password_too_short")
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
	if utf8.RuneCountInString(next) < MinPasswordLength {
		var v apperr.Validation
		v.Add("new_password", "validation.password_too_short")
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

// PreferencesPatch updates the caller's own display settings; nil fields stay.
type PreferencesPatch struct {
	DisplayName    *string
	Locale         *string
	DateFormat     *string
	NumberFormat   *string
	CurrencyCode   *string
	CurrencySymbol *string // "" clears the override
	PowerUnit      *string
}

// UpdatePreferences validates and stores the caller's preferences.
func (u *Users) UpdatePreferences(ctx context.Context, userID int64, p PreferencesPatch) (User, error) {
	cur, err := store.New(u.DB.Reader).GetUser(ctx, userID)
	if err != nil {
		return User{}, err
	}
	next := store.UpdateUserPreferencesParams{
		DisplayName: cur.DisplayName, Locale: cur.Locale, DateFormat: cur.DateFormat, NumberFormat: cur.NumberFormat,
		CurrencyCode: cur.CurrencyCode, CurrencySymbol: cur.CurrencySymbol, PowerUnit: cur.PowerUnit,
		UpdatedAt: clock.Timestamp(u.Clock.Now()), ID: userID,
	}
	var v apperr.Validation
	if p.DisplayName != nil {
		validateDisplayName(&v, *p.DisplayName)
		next.DisplayName = strings.TrimSpace(*p.DisplayName)
	}
	if p.Locale != nil {
		v.Check(*p.Locale != "en" && *p.Locale != "de", "locale", "validation.invalid_choice")
		next.Locale = *p.Locale
	}
	if p.DateFormat != nil {
		v.Check(*p.DateFormat != "DD.MM.YYYY" && *p.DateFormat != "YYYY-MM-DD", "date_format", "validation.invalid_choice")
		next.DateFormat = *p.DateFormat
	}
	if p.NumberFormat != nil {
		v.Check(*p.NumberFormat != "point" && *p.NumberFormat != "comma", "number_format", "validation.invalid_choice")
		next.NumberFormat = *p.NumberFormat
	}
	if p.CurrencyCode != nil {
		code := strings.ToUpper(strings.TrimSpace(*p.CurrencyCode))
		v.Check(!currencyCodeRe.MatchString(code), "currency_code", "validation.currency_code")
		next.CurrencyCode = code
	}
	if p.CurrencySymbol != nil {
		sym := strings.TrimSpace(*p.CurrencySymbol)
		v.Check(utf8.RuneCountInString(sym) > 5, "currency_symbol", "validation.length")
		next.CurrencySymbol = sql.NullString{String: sym, Valid: sym != ""}
	}
	if p.PowerUnit != nil {
		v.Check(*p.PowerUnit != "kw" && *p.PowerUnit != "ps", "power_unit", "validation.invalid_choice")
		next.PowerUnit = *p.PowerUnit
	}
	if err := v.Err(); err != nil {
		return User{}, err
	}
	row, err := store.New(u.DB.Writer).UpdateUserPreferences(ctx, next)
	if err != nil {
		return User{}, err
	}
	return FromStore(row), nil
}
