package httpapi

import (
	"encoding/json"
	"math"
	"net/http"
	"slices"
	"strings"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/auth"
	"github.com/app-torq/torq/internal/auth/mfa"
	"github.com/app-torq/torq/internal/store"
)

// startChallenge answers a correct password of a user with a second factor:
// no session, but a challenge to complete with that factor (research T6).
// It returns nil when the user has no second factor.
func (s *Server) startChallenge(r *http.Request, userID int64, in signIn) error {
	methods, err := s.mfa.Methods(r.Context(), userID)
	if err != nil || len(methods) == 0 {
		return err
	}
	id := s.mfa.Challenges.Put(&mfa.Challenge{UserID: userID, Methods: methods, Token: in.Token, DeviceName: in.DeviceName})
	return &apperr.Error{Status: http.StatusUnauthorized, Code: "auth.mfa_required", Params: map[string]any{"challenge": id, "methods": methods}}
}

// throttleKeys are the login throttle keys for a user signing in from r.
func (s *Server) throttleKeys(r *http.Request, username string) (ipKeys, userKeys []string) {
	ip := "ip:" + auth.ClientIP(r, s.Config.TrustedProxies)
	user := "user:" + strings.ToLower(username)
	return []string{ip, user + "@" + ip}, []string{user}
}

// finishSignIn starts the session of a user who passed every factor and
// writes the sign-in response.
func (s *Server) finishSignIn(w http.ResponseWriter, r *http.Request, user auth.User, in signIn) error {
	if user.Disabled {
		return &apperr.Error{Status: http.StatusUnauthorized, Code: "auth.invalid_credentials"}
	}
	token, err := s.startSession(w, r, user.ID, in)
	if err != nil {
		return err
	}
	info, err := s.instanceInfo(r.Context(), true)
	if err != nil {
		return err
	}
	out, err := s.meOut(r.Context(), user)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, meResponse{User: out, Instance: info, Token: token})
	return nil
}

// completeMFA answers a sign-in challenge with a code, a recovery code or a
// passkey.
func (s *Server) completeMFA(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Challenge    string          `json:"challenge"`
		Code         string          `json:"code"`
		RecoveryCode string          `json:"recovery_code"`
		Passkey      json.RawMessage `json:"passkey"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	ch, ok := s.mfa.Challenges.Get(in.Challenge)
	if !ok {
		return mfa.ChallengeExpired()
	}
	user, err := s.users.Get(ctx, ch.UserID)
	if err != nil {
		return err
	}
	ipKeys, userKeys := s.throttleKeys(r, user.Username)
	if wait := max(s.throttle.RetryAfter(ipKeys...), s.userThrottle.RetryAfter(userKeys...)); wait > 0 {
		return apperr.Throttled(int(math.Ceil(wait.Seconds())))
	}
	var passed bool
	switch {
	case in.Code != "" && slices.Contains(ch.Methods, mfa.MethodTOTP):
		passed, err = s.mfa.VerifyTOTP(ctx, ch.UserID, in.Code)
	case in.RecoveryCode != "" && slices.Contains(ch.Methods, mfa.MethodRecovery):
		passed, err = s.mfa.UseRecoveryCode(ctx, ch.UserID, in.RecoveryCode)
	case len(in.Passkey) > 0 && slices.Contains(ch.Methods, mfa.MethodPasskey):
		passed, err = s.mfa.VerifyChallengePasskey(ctx, ch, in.Passkey)
	}
	if err != nil {
		return err
	}
	if !passed {
		s.throttle.Fail(ipKeys...)
		s.userThrottle.Fail(userKeys...)
		left := s.mfa.Challenges.Fail(in.Challenge)
		if left == 0 {
			return mfa.ChallengeExpired()
		}
		return &apperr.Error{Status: http.StatusUnauthorized, Code: "auth.mfa_invalid", Params: map[string]any{"attempts_left": left}}
	}
	if _, ok := s.mfa.Challenges.Take(in.Challenge); !ok {
		return mfa.ChallengeExpired()
	}
	s.throttle.Succeed(ipKeys[1:]...)
	s.userThrottle.Succeed(userKeys...)
	return s.finishSignIn(w, r, user, signIn{Token: ch.Token, DeviceName: ch.DeviceName})
}

// challengePasskeyOptions starts the passkey check of a sign-in challenge.
func (s *Server) challengePasskeyOptions(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Challenge string `json:"challenge"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	opts, err := s.mfa.BeginChallengePasskey(r.Context(), in.Challenge)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"options": opts})
	return nil
}

// passkeySignInOptions starts a sign-in with a passkey and no username.
func (s *Server) passkeySignInOptions(w http.ResponseWriter, _ *http.Request) error {
	id, opts, err := s.mfa.BeginPasskeySignIn()
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"session": id, "options": opts})
	return nil
}

// passkeySignIn completes a sign-in with a passkey and no password; a passkey
// that verified its user counts as both factors (research T4).
func (s *Server) passkeySignIn(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Session    string          `json:"session"`
		Credential json.RawMessage `json:"credential"`
		signIn
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	ip := "ip:" + auth.ClientIP(r, s.Config.TrustedProxies)
	if wait := s.throttle.RetryAfter(ip); wait > 0 {
		return apperr.Throttled(int(math.Ceil(wait.Seconds())))
	}
	userID, err := s.mfa.FinishPasskeySignIn(r.Context(), in.Session, in.Credential)
	if e, ok := apperr.As(err); ok && e.Status == http.StatusUnauthorized {
		s.throttle.Fail(ip)
	}
	if err != nil {
		return err
	}
	user, err := s.users.Get(r.Context(), userID)
	if err != nil {
		return err
	}
	return s.finishSignIn(w, r, user, in.signIn)
}

type passkeyJSON struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	UserVerified bool    `json:"user_verified"`
	CreatedAt    string  `json:"created_at"`
	LastUsedAt   *string `json:"last_used_at"`
}

func toPasskeyJSON(p store.UserPasskey) passkeyJSON {
	return passkeyJSON{ID: p.ID, Name: p.Name, UserVerified: p.UserVerified == 1, CreatedAt: p.CreatedAt, LastUsedAt: strPtr(p.LastUsedAt)}
}

// getMFA is the caller's second-factor overview.
func (s *Server) getMFA(w http.ResponseWriter, r *http.Request) error {
	me := currentUser(r)
	st, err := s.mfa.Status(r.Context(), me.ID)
	if err != nil {
		return err
	}
	keys := make([]passkeyJSON, 0, len(st.Passkeys))
	for _, p := range st.Passkeys {
		keys = append(keys, toPasskeyJSON(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"totp": st.TOTP, "recovery_codes_left": st.RecoveryLeft, "passkeys": keys,
		"required": me.Access.MFARequired, "passkeys_available": s.mfa.Passkeys != nil,
	})
	return nil
}

// beginTOTP returns a new secret for the authenticator app.
func (s *Server) beginTOTP(w http.ResponseWriter, r *http.Request) error {
	me := currentUser(r)
	secret, err := s.mfa.BeginTOTP(r.Context(), me.ID)
	if err != nil {
		return err
	}
	b, err := s.branding(r.Context(), false)
	if err != nil {
		return err
	}
	issuer := b.Name
	if issuer == "" {
		issuer = "torq"
	}
	writeJSON(w, http.StatusOK, map[string]string{"secret": mfa.SecretText(secret), "uri": mfa.TOTPURI(issuer, me.Username, secret)})
	return nil
}

func writeRecoveryCodes(w http.ResponseWriter, status int, codes []string, extra map[string]any) {
	if codes == nil {
		codes = []string{}
	}
	body := map[string]any{"recovery_codes": codes}
	for k, v := range extra {
		body[k] = v
	}
	writeJSON(w, status, body)
}

func (s *Server) confirmTOTP(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	codes, err := s.mfa.ConfirmTOTP(r.Context(), currentUser(r).ID, in.Code)
	if err != nil {
		return err
	}
	writeRecoveryCodes(w, http.StatusOK, codes, nil)
	return nil
}

func (s *Server) deleteTOTP(w http.ResponseWriter, r *http.Request) error {
	me := currentUser(r)
	if err := s.mfa.DeleteTOTP(r.Context(), me.ID, me.Access.MFARequired); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) newRecoveryCodes(w http.ResponseWriter, r *http.Request) error {
	codes, err := s.mfa.NewRecoveryCodes(r.Context(), currentUser(r).ID)
	if err != nil {
		return err
	}
	writeRecoveryCodes(w, http.StatusOK, codes, nil)
	return nil
}

func (s *Server) passkeyRegistrationOptions(w http.ResponseWriter, r *http.Request) error {
	id, opts, err := s.mfa.BeginPasskeyRegistration(r.Context(), currentUser(r).ID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"session": id, "options": opts})
	return nil
}

func (s *Server) addPasskey(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Session    string          `json:"session"`
		Name       string          `json:"name"`
		Credential json.RawMessage `json:"credential"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	p, codes, err := s.mfa.FinishPasskeyRegistration(r.Context(), currentUser(r).ID, in.Session, in.Name, in.Credential)
	if err != nil {
		return err
	}
	writeRecoveryCodes(w, http.StatusCreated, codes, map[string]any{"passkey": toPasskeyJSON(p)})
	return nil
}

func (s *Server) renamePasskey(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	p, err := s.mfa.RenamePasskey(r.Context(), currentUser(r).ID, id, in.Name)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"passkey": toPasskeyJSON(p)})
	return nil
}

func (s *Server) deletePasskey(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	me := currentUser(r)
	if err := s.mfa.DeletePasskey(r.Context(), me.ID, id, me.Access.MFARequired); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// resetUserMFA removes another user's second factors (administrators).
func (s *Server) resetUserMFA(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if _, err := s.users.Get(r.Context(), id); err != nil {
		return err
	}
	if err := s.mfa.Reset(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
