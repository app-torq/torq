package mfa

import (
	"context"
	"database/sql"
	"errors"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/db"
	"github.com/app-torq/torq/internal/store"
)

// Sign-in methods a challenge can be answered with.
const (
	MethodTOTP     = "totp"
	MethodRecovery = "recovery_code"
	MethodPasskey  = "passkey"
)

// Challenge is a password sign-in waiting for its second factor.
type Challenge struct {
	UserID  int64
	Methods []string
	// Token and DeviceName are the sign-in options of the password step.
	Token      bool
	DeviceName string
	// passkey is the WebAuthn ceremony started for this challenge, if any.
	passkey *ceremony
}

// Service manages a user's second factors and sign-in challenges.
type Service struct {
	DB    *db.DB
	Clock clock.Clock
	Box   *Box
	// Passkeys is nil when the server's address isn't configured.
	Passkeys   *Passkeys
	Challenges *Store[*Challenge]
}

// New returns a service; passkeys may be nil.
func New(d *db.DB, clk clock.Clock, box *Box, passkeys *Passkeys) *Service {
	return &Service{DB: d, Clock: clk, Box: box, Passkeys: passkeys, Challenges: NewStore[*Challenge](clk.Now)}
}

// Status is what a user has set up.
type Status struct {
	TOTP         bool
	RecoveryLeft int64
	Passkeys     []store.UserPasskey
}

// Factors reports whether the user has any second factor (q may be a
// transaction or the reader).
func Factors(ctx context.Context, q *store.Queries, userID int64) (int64, error) {
	return q.CountFactors(ctx, userID)
}

// Status returns the user's second factors.
func (s *Service) Status(ctx context.Context, userID int64) (Status, error) {
	q := store.New(s.DB.Reader)
	var st Status
	t, err := q.GetTOTP(ctx, userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return st, err
	}
	st.TOTP = err == nil && t.ConfirmedAt.Valid
	if st.RecoveryLeft, err = q.CountRecoveryCodes(ctx, userID); err != nil {
		return st, err
	}
	st.Passkeys, err = q.ListPasskeys(ctx, userID)
	return st, err
}

// Methods returns how the user can answer a sign-in challenge; empty when
// they have no second factor.
func (s *Service) Methods(ctx context.Context, userID int64) ([]string, error) {
	st, err := s.Status(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []string
	if st.TOTP {
		out = append(out, MethodTOTP)
	}
	if len(st.Passkeys) > 0 && s.Passkeys != nil {
		out = append(out, MethodPasskey)
	}
	if len(out) > 0 && st.RecoveryLeft > 0 {
		out = append(out, MethodRecovery)
	}
	if len(out) == 0 && len(st.Passkeys) > 0 {
		// Passkeys exist but the server's address changed or went away:
		// the recovery codes are the way in.
		out = append(out, MethodRecovery)
	}
	return out, nil
}

// BeginTOTP starts (or restarts) setting up an authenticator app and returns
// the secret.
func (s *Service) BeginTOTP(ctx context.Context, userID int64) ([]byte, error) {
	q := store.New(s.DB.Writer)
	if t, err := q.GetTOTP(ctx, userID); err == nil && t.ConfirmedAt.Valid {
		return nil, apperr.Conflict("mfa.totp_enabled", nil)
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	secret := NewTOTPSecret()
	err := q.UpsertPendingTOTP(ctx, store.UpsertPendingTOTPParams{
		UserID: userID, SecretEnc: s.Box.Seal(userID, secret), CreatedAt: clock.Timestamp(s.Clock.Now()),
	})
	return secret, err
}

func codeInvalid() error {
	var v apperr.Validation
	v.Add("code", "mfa.code_invalid")
	return v.Err()
}

// ConfirmTOTP turns the pending authenticator on with a valid code. When it
// is the user's first factor, it returns their new recovery codes.
func (s *Service) ConfirmTOTP(ctx context.Context, userID int64, code string) ([]string, error) {
	var codes []string
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		t, err := q.GetTOTP(ctx, userID)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.Conflict("mfa.no_setup", nil)
		}
		if err != nil {
			return err
		}
		if t.ConfirmedAt.Valid {
			return apperr.Conflict("mfa.totp_enabled", nil)
		}
		secret, err := s.Box.Open(userID, t.SecretEnc)
		if err != nil {
			return err
		}
		step, ok := MatchStep(secret, code, s.Clock.Now(), 0)
		if !ok {
			return codeInvalid()
		}
		n, err := q.CountFactors(ctx, userID)
		if err != nil {
			return err
		}
		now := clock.Timestamp(s.Clock.Now())
		if err := q.ConfirmTOTP(ctx, store.ConfirmTOTPParams{ConfirmedAt: sql.NullString{String: now, Valid: true}, LastStep: step, UserID: userID}); err != nil {
			return err
		}
		if n == 0 {
			codes, err = replaceRecoveryCodes(ctx, q, userID, now)
		}
		return err
	})
	return codes, err
}

// required reports whether removing a factor would leave a user who must
// have one without any.
func ensureNotLast(ctx context.Context, q *store.Queries, userID int64, required bool) error {
	if !required {
		return nil
	}
	n, err := q.CountFactors(ctx, userID)
	if err != nil {
		return err
	}
	if n <= 1 {
		return apperr.Conflict("mfa.required", nil)
	}
	return nil
}

// DeleteTOTP turns the authenticator off. required says whether the user must
// keep a second factor.
func (s *Service) DeleteTOTP(ctx context.Context, userID int64, required bool) error {
	return s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		t, err := q.GetTOTP(ctx, userID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if t.ConfirmedAt.Valid {
			if err := ensureNotLast(ctx, q, userID, required); err != nil {
				return err
			}
		}
		if err := q.DeleteTOTP(ctx, userID); err != nil {
			return err
		}
		return dropCodesWithoutFactor(ctx, q, userID)
	})
}

// dropCodesWithoutFactor removes recovery codes once no factor is left, so a
// later setup starts with a fresh set.
func dropCodesWithoutFactor(ctx context.Context, q *store.Queries, userID int64) error {
	n, err := q.CountFactors(ctx, userID)
	if err != nil || n > 0 {
		return err
	}
	return q.DeleteRecoveryCodes(ctx, userID)
}

func replaceRecoveryCodes(ctx context.Context, q *store.Queries, userID int64, now string) ([]string, error) {
	if err := q.DeleteRecoveryCodes(ctx, userID); err != nil {
		return nil, err
	}
	codes := NewRecoveryCodes()
	for _, c := range codes {
		if err := q.InsertRecoveryCode(ctx, store.InsertRecoveryCodeParams{UserID: userID, CodeHash: HashRecoveryCode(c), CreatedAt: now}); err != nil {
			return nil, err
		}
	}
	return codes, nil
}

// NewRecoveryCodes replaces the user's recovery codes.
func (s *Service) NewRecoveryCodes(ctx context.Context, userID int64) ([]string, error) {
	var codes []string
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		n, err := q.CountFactors(ctx, userID)
		if err != nil {
			return err
		}
		if n == 0 {
			return apperr.Conflict("mfa.no_factor", nil)
		}
		codes, err = replaceRecoveryCodes(ctx, q, userID, clock.Timestamp(s.Clock.Now()))
		return err
	})
	return codes, err
}

// VerifyTOTP checks a code from the authenticator app; each code works once.
func (s *Service) VerifyTOTP(ctx context.Context, userID int64, code string) (bool, error) {
	q := store.New(s.DB.Writer)
	t, err := q.GetTOTP(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !t.ConfirmedAt.Valid) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	secret, err := s.Box.Open(userID, t.SecretEnc)
	if err != nil {
		return false, err
	}
	step, ok := MatchStep(secret, code, s.Clock.Now(), t.LastStep)
	if !ok {
		return false, nil
	}
	n, err := q.SetTOTPStep(ctx, store.SetTOTPStepParams{Step: step, UserID: userID})
	return n == 1, err
}

// UseRecoveryCode accepts an unused recovery code once.
func (s *Service) UseRecoveryCode(ctx context.Context, userID int64, code string) (bool, error) {
	n, err := store.New(s.DB.Writer).UseRecoveryCode(ctx, store.UseRecoveryCodeParams{
		UsedAt: sql.NullString{String: clock.Timestamp(s.Clock.Now()), Valid: true}, UserID: userID, CodeHash: HashRecoveryCode(code),
	})
	return n == 1, err
}

// Reset removes every second factor of a user and ends their sessions.
func (s *Service) Reset(ctx context.Context, userID int64) error {
	return s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		for _, del := range []func(context.Context, int64) error{
			q.DeleteTOTP, q.DeleteRecoveryCodes, q.DeletePasskeys, q.DeleteWebAuthnHandle, q.DeleteUserSessions,
		} {
			if err := del(ctx, userID); err != nil {
				return err
			}
		}
		return nil
	})
}
