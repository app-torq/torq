package mfa

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/store"
)

// Passkeys runs WebAuthn ceremonies for one server address.
type Passkeys struct {
	wa         *webauthn.WebAuthn
	ceremonies *Store[*ceremony]
}

// ceremony is a started registration or sign-in.
type ceremony struct {
	userID  int64 // 0 for a sign-in without username
	session webauthn.SessionData
}

// NewPasskeys returns nil when baseURL is empty: passkeys are bound to the
// server's address, so they need TORQ_BASE_URL.
func NewPasskeys(baseURL, displayName string, clk clock.Clock) (*Passkeys, error) {
	if baseURL == "" {
		return nil, nil
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	wa, err := webauthn.New(&webauthn.Config{RPID: u.Hostname(), RPDisplayName: displayName, RPOrigins: []string{baseURL}})
	if err != nil {
		return nil, err
	}
	return &Passkeys{wa: wa, ceremonies: NewStore[*ceremony](clk.Now)}, nil
}

// waUser adapts a torq user to the library.
type waUser struct {
	handle      []byte
	name        string
	displayName string
	creds       []webauthn.Credential
}

func (u *waUser) WebAuthnID() []byte                         { return u.handle }
func (u *waUser) WebAuthnName() string                       { return u.name }
func (u *waUser) WebAuthnDisplayName() string                { return u.displayName }
func (u *waUser) WebAuthnCredentials() []webauthn.Credential { return u.creds }

func unavailable() error { return apperr.Conflict("passkey.unavailable", nil) }

func passkeyInvalid(status int) error {
	return &apperr.Error{Status: status, Code: "passkey.invalid"}
}

// loadUser returns the library's view of a user, creating their handle when
// create is set.
func (s *Service) loadUser(ctx context.Context, q *store.Queries, userID int64, create bool) (*waUser, error) {
	u, err := q.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	handle, err := q.GetWebAuthnHandle(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) && create {
		handle = make([]byte, 32)
		_, _ = rand.Read(handle)
		err = q.InsertWebAuthnHandle(ctx, store.InsertWebAuthnHandleParams{UserID: userID, Handle: handle})
	}
	if err != nil {
		return nil, err
	}
	rows, err := q.ListPasskeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	wu := &waUser{handle: handle, name: u.Username, displayName: u.DisplayName}
	for _, r := range rows {
		var c webauthn.Credential
		if err := json.Unmarshal([]byte(r.Credential), &c); err != nil {
			return nil, err
		}
		wu.creds = append(wu.creds, c)
	}
	return wu, nil
}

// BeginPasskeyRegistration starts adding a passkey and returns the ceremony id
// and the options for the browser.
func (s *Service) BeginPasskeyRegistration(ctx context.Context, userID int64) (string, *protocol.CredentialCreation, error) {
	if s.Passkeys == nil {
		return "", nil, unavailable()
	}
	var id string
	var opts *protocol.CredentialCreation
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		wu, err := s.loadUser(ctx, store.New(tx), userID, true)
		if err != nil {
			return err
		}
		exclude := make([]protocol.CredentialDescriptor, 0, len(wu.creds))
		for _, c := range wu.creds {
			exclude = append(exclude, c.Descriptor())
		}
		var session *webauthn.SessionData
		opts, session, err = s.Passkeys.wa.BeginRegistration(wu,
			webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
				ResidentKey: protocol.ResidentKeyRequirementRequired, UserVerification: protocol.VerificationPreferred,
			}),
			webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
			webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
			webauthn.WithExclusions(exclude),
		)
		if err != nil {
			return err
		}
		id = s.Passkeys.ceremonies.Put(&ceremony{userID: userID, session: *session})
		return nil
	})
	return id, opts, err
}

// NormalizePasskeyName trims a passkey name and checks its length.
func NormalizePasskeyName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 64 {
		var v apperr.Validation
		v.Add("name", "validation.length")
		return "", v.Err()
	}
	return name, nil
}

// FinishPasskeyRegistration verifies the browser's answer and stores the
// passkey. When it is the user's first factor, it returns new recovery codes.
func (s *Service) FinishPasskeyRegistration(ctx context.Context, userID int64, ceremonyID, name string, credential json.RawMessage) (store.UserPasskey, []string, error) {
	if s.Passkeys == nil {
		return store.UserPasskey{}, nil, unavailable()
	}
	name, err := NormalizePasskeyName(name)
	if err != nil {
		return store.UserPasskey{}, nil, err
	}
	c, ok := s.Passkeys.ceremonies.Take(ceremonyID)
	if !ok || c.userID != userID {
		return store.UserPasskey{}, nil, passkeyInvalid(422)
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(credential)
	if err != nil {
		return store.UserPasskey{}, nil, passkeyInvalid(422)
	}
	var out store.UserPasskey
	var codes []string
	err = s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		wu, err := s.loadUser(ctx, q, userID, false)
		if err != nil {
			return err
		}
		cred, err := s.Passkeys.wa.CreateCredential(wu, c.session, parsed)
		if err != nil {
			return passkeyInvalid(422)
		}
		raw, err := json.Marshal(cred)
		if err != nil {
			return err
		}
		n, err := q.CountFactors(ctx, userID)
		if err != nil {
			return err
		}
		now := clock.Timestamp(s.Clock.Now())
		uv := int64(0)
		if cred.Flags.UserVerified {
			uv = 1
		}
		out, err = q.InsertPasskey(ctx, store.InsertPasskeyParams{
			UserID: userID, CredentialID: cred.ID, Credential: string(raw), Name: name, UserVerified: uv, CreatedAt: now,
		})
		if err != nil {
			return err
		}
		if n == 0 {
			codes, err = replaceRecoveryCodes(ctx, q, userID, now)
		}
		return err
	})
	return out, codes, err
}

// RenamePasskey renames one of the user's passkeys.
func (s *Service) RenamePasskey(ctx context.Context, userID, id int64, name string) (store.UserPasskey, error) {
	name, err := NormalizePasskeyName(name)
	if err != nil {
		return store.UserPasskey{}, err
	}
	p, err := store.New(s.DB.Writer).RenamePasskey(ctx, store.RenamePasskeyParams{Name: name, ID: id, UserID: userID})
	if errors.Is(err, sql.ErrNoRows) {
		return p, apperr.NotFound()
	}
	return p, err
}

// DeletePasskey removes one of the user's passkeys. required says whether the
// user must keep a second factor.
func (s *Service) DeletePasskey(ctx context.Context, userID, id int64, required bool) error {
	return s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if _, err := q.GetPasskey(ctx, store.GetPasskeyParams{ID: id, UserID: userID}); errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		} else if err != nil {
			return err
		}
		if err := ensureNotLast(ctx, q, userID, required); err != nil {
			return err
		}
		if _, err := q.DeletePasskey(ctx, store.DeletePasskeyParams{ID: id, UserID: userID}); err != nil {
			return err
		}
		return dropCodesWithoutFactor(ctx, q, userID)
	})
}

// BeginChallengePasskey starts a passkey check for a sign-in challenge (the
// passkey as second factor after the password).
func (s *Service) BeginChallengePasskey(ctx context.Context, challengeID string) (*protocol.CredentialAssertion, error) {
	if s.Passkeys == nil {
		return nil, unavailable()
	}
	ch, ok := s.Challenges.Get(challengeID)
	if !ok {
		return nil, ChallengeExpired()
	}
	wu, err := s.loadUser(ctx, store.New(s.DB.Reader), ch.UserID, false)
	if err != nil {
		return nil, err
	}
	if len(wu.creds) == 0 {
		return nil, passkeyInvalid(401)
	}
	opts, session, err := s.Passkeys.wa.BeginLogin(wu, webauthn.WithUserVerification(protocol.VerificationPreferred))
	if err != nil {
		return nil, err
	}
	ch.passkey = &ceremony{userID: ch.UserID, session: *session}
	return opts, nil
}

// VerifyChallengePasskey checks the passkey answer of a sign-in challenge.
func (s *Service) VerifyChallengePasskey(ctx context.Context, ch *Challenge, credential json.RawMessage) (bool, error) {
	if s.Passkeys == nil || ch.passkey == nil {
		return false, nil
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(credential)
	if err != nil {
		return false, nil
	}
	wu, err := s.loadUser(ctx, store.New(s.DB.Reader), ch.UserID, false)
	if err != nil {
		return false, err
	}
	cred, err := s.Passkeys.wa.ValidateLogin(wu, ch.passkey.session, parsed)
	if err != nil {
		return false, nil
	}
	return true, s.recordUse(ctx, ch.UserID, cred)
}

// BeginPasskeySignIn starts a sign-in without username: any passkey of this
// server that verifies its user.
func (s *Service) BeginPasskeySignIn() (string, *protocol.CredentialAssertion, error) {
	if s.Passkeys == nil {
		return "", nil, unavailable()
	}
	opts, session, err := s.Passkeys.wa.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return "", nil, err
	}
	return s.Passkeys.ceremonies.Put(&ceremony{session: *session}), opts, nil
}

// FinishPasskeySignIn verifies a sign-in without username and returns the
// user. The library refuses answers without user verification, so the
// passkey counts as both factors.
func (s *Service) FinishPasskeySignIn(ctx context.Context, ceremonyID string, credential json.RawMessage) (int64, error) {
	if s.Passkeys == nil {
		return 0, unavailable()
	}
	c, ok := s.Passkeys.ceremonies.Take(ceremonyID)
	if !ok || c.userID != 0 {
		return 0, passkeyInvalid(401)
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(credential)
	if err != nil {
		return 0, passkeyInvalid(401)
	}
	if !parsed.Response.AuthenticatorData.Flags.HasUserVerified() {
		return 0, &apperr.Error{Status: 401, Code: "passkey.verification_required"}
	}
	q := store.New(s.DB.Reader)
	var userID int64
	handler := func(_, userHandle []byte) (webauthn.User, error) {
		id, err := q.GetUserByWebAuthnHandle(ctx, userHandle)
		if err != nil {
			return nil, err
		}
		userID = id
		return s.loadUser(ctx, q, id, false)
	}
	_, cred, err := s.Passkeys.wa.ValidatePasskeyLogin(handler, c.session, parsed)
	if err != nil || userID == 0 {
		return 0, passkeyInvalid(401)
	}
	return userID, s.recordUse(ctx, userID, cred)
}

// recordUse stores the updated counter and flags and the time of use.
func (s *Service) recordUse(ctx context.Context, userID int64, cred *webauthn.Credential) error {
	q := store.New(s.DB.Writer)
	rows, err := q.ListPasskeys(ctx, userID)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(cred)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if string(r.CredentialID) == string(cred.ID) {
			return q.UpdatePasskeyUse(ctx, store.UpdatePasskeyUseParams{
				Credential: string(raw), LastUsedAt: sql.NullString{String: clock.Timestamp(s.Clock.Now()), Valid: true}, ID: r.ID,
			})
		}
	}
	return nil
}

// ChallengeExpired is the error for an unknown, used up or expired challenge.
func ChallengeExpired() error {
	return &apperr.Error{Status: 401, Code: "auth.mfa_challenge_expired"}
}
