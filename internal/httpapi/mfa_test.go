package httpapi

import (
	"encoding/base32"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/app-torq/torq/internal/auth/mfa"
	"github.com/app-torq/torq/internal/config"
)

type mfaError struct {
	Error struct {
		Code   string         `json:"code"`
		Params map[string]any `json:"params"`
	} `json:"error"`
}

// setUpTOTP turns on an authenticator app for c and returns its secret and
// recovery codes.
func setUpTOTP(t *testing.T, e *env, c *client) ([]byte, []string) {
	t.Helper()
	var begin struct {
		Secret string `json:"secret"`
		URI    string `json:"uri"`
	}
	c.call(http.MethodPost, "/me/totp", nil, http.StatusOK, &begin)
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(begin.Secret)
	if err != nil || !strings.HasPrefix(begin.URI, "otpauth://totp/torq:") {
		t.Fatalf("setup %+v %v", begin, err)
	}
	var codes struct {
		RecoveryCodes []string `json:"recovery_codes"`
	}
	c.call(http.MethodPost, "/me/totp/confirm", map[string]string{"code": mfa.CodeAt(secret, mfa.Step(e.clock.Now()))}, http.StatusOK, &codes)
	if len(codes.RecoveryCodes) != mfa.RecoveryCodeCount {
		t.Fatalf("recovery codes %v", codes.RecoveryCodes)
	}
	return secret, codes.RecoveryCodes
}

// challenge signs in with the password and returns the second-factor
// challenge.
func challenge(t *testing.T, e *env, username string, body map[string]any) (string, []any) {
	t.Helper()
	if body == nil {
		body = map[string]any{}
	}
	body["username"], body["password"] = username, testPassword
	rec := e.client().do(http.MethodPost, "/auth/login", body)
	var out mfaError
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if rec.Code != http.StatusUnauthorized || out.Error.Code != "auth.mfa_required" {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Set-Cookie") != "" || strings.Contains(rec.Body.String(), `"token"`) {
		t.Fatal("password alone started a session")
	}
	id, _ := out.Error.Params["challenge"].(string)
	methods, _ := out.Error.Params["methods"].([]any)
	return id, methods
}

func TestTOTPSignIn(t *testing.T) {
	e := newEnv(t)
	adm := e.admin()
	var me meResp
	adm.call(http.MethodGet, "/auth/me", nil, http.StatusOK, &me)
	if me.User.MFAEnabled {
		t.Fatal("2FA on before setup")
	}
	secret, codes := setUpTOTP(t, e, adm)
	adm.call(http.MethodGet, "/auth/me", nil, http.StatusOK, &me)
	if !me.User.MFAEnabled {
		t.Fatal("2FA not reported after setup")
	}
	if code := adm.errorCode(http.MethodPost, "/me/totp", nil, http.StatusConflict); code != "mfa.totp_enabled" {
		t.Fatalf("code %q", code)
	}

	// The code used for setup can't be used again; the next one works once.
	e.clock.Advance(30 * time.Second)
	id, methods := challenge(t, e, "marek", nil)
	if len(methods) != 2 || methods[0] != "totp" || methods[1] != "recovery_code" {
		t.Fatalf("methods %v", methods)
	}
	c := e.client()
	code := mfa.CodeAt(secret, mfa.Step(e.clock.Now()))
	if got := c.errorCode(http.MethodPost, "/auth/mfa", map[string]any{"challenge": id, "code": "000000"}, http.StatusUnauthorized); got != "auth.mfa_invalid" {
		t.Fatalf("wrong code: %q", got)
	}
	c.call(http.MethodPost, "/auth/mfa", map[string]any{"challenge": id, "code": code}, http.StatusOK, &me)
	if c.cookie == nil || me.User.Username != "marek" {
		t.Fatal("no session after the second factor")
	}
	c.call(http.MethodGet, "/vehicles", nil, http.StatusOK, nil)
	if got := e.client().errorCode(http.MethodPost, "/auth/mfa", map[string]any{"challenge": id, "code": code}, http.StatusUnauthorized); got != "auth.mfa_challenge_expired" {
		t.Fatalf("reused challenge: %q", got)
	}
	id, _ = challenge(t, e, "marek", nil)
	if got := e.client().errorCode(http.MethodPost, "/auth/mfa", map[string]any{"challenge": id, "code": code}, http.StatusUnauthorized); got != "auth.mfa_invalid" {
		t.Fatalf("reused code: %q", got)
	}

	// Recovery codes work once, also for apps asking for a token.
	id, _ = challenge(t, e, "marek", map[string]any{"token": true, "device_name": "Phone"})
	var tok struct {
		Token string `json:"token"`
	}
	e.client().call(http.MethodPost, "/auth/mfa", map[string]any{"challenge": id, "recovery_code": strings.ToUpper(codes[0])}, http.StatusOK, &tok)
	if tok.Token == "" {
		t.Fatal("no token for the app")
	}
	(&client{e: e, token: tok.Token}).call(http.MethodGet, "/auth/me", nil, http.StatusOK, nil)
	id, _ = challenge(t, e, "marek", nil)
	if got := e.client().errorCode(http.MethodPost, "/auth/mfa", map[string]any{"challenge": id, "recovery_code": codes[0]}, http.StatusUnauthorized); got != "auth.mfa_invalid" {
		t.Fatalf("reused recovery code: %q", got)
	}
	var status struct {
		RecoveryCodesLeft int64 `json:"recovery_codes_left"`
		TOTP              bool  `json:"totp"`
	}
	adm.call(http.MethodGet, "/me/mfa", nil, http.StatusOK, &status)
	if status.RecoveryCodesLeft != 9 || !status.TOTP {
		t.Fatalf("status %+v", status)
	}

	// New codes replace the old set.
	var fresh struct {
		RecoveryCodes []string `json:"recovery_codes"`
	}
	adm.call(http.MethodPost, "/me/recovery-codes", nil, http.StatusOK, &fresh)
	id, _ = challenge(t, e, "marek", nil)
	if got := e.client().errorCode(http.MethodPost, "/auth/mfa", map[string]any{"challenge": id, "recovery_code": codes[1]}, http.StatusUnauthorized); got != "auth.mfa_invalid" {
		t.Fatalf("old set: %q", got)
	}

	// Turning it off brings back password-only sign-in.
	adm.call(http.MethodDelete, "/me/totp", nil, http.StatusNoContent, nil)
	e.login("marek", testPassword)
}

func TestMFAChallengeLimits(t *testing.T) {
	e := newEnv(t)
	adm := e.admin()
	setUpTOTP(t, e, adm)

	id, _ := challenge(t, e, "marek", nil)
	c := e.clientFrom("192.0.2.7")
	for i := 1; i < mfa.ChallengeAttempts; i++ {
		var out mfaError
		c.call(http.MethodPost, "/auth/mfa", map[string]any{"challenge": id, "code": "111111"}, http.StatusUnauthorized, &out)
		if out.Error.Code != "auth.mfa_invalid" || out.Error.Params["attempts_left"] != float64(mfa.ChallengeAttempts-i) {
			t.Fatalf("attempt %d: %+v", i, out.Error)
		}
		if i >= 4 {
			break // the throttle takes over before the challenge runs out
		}
	}
	if rec := c.do(http.MethodPost, "/auth/mfa", map[string]any{"challenge": id, "code": "111111"}); rec.Code != http.StatusTooManyRequests && rec.Code != http.StatusUnauthorized {
		t.Fatalf("after many failures: %d %s", rec.Code, rec.Body)
	}

	// Challenges expire after five minutes.
	id, _ = challenge(t, e, "marek", nil)
	e.clock.Advance(mfa.ChallengeTTL)
	if got := e.clientFrom("192.0.2.8").errorCode(http.MethodPost, "/auth/mfa", map[string]any{"challenge": id, "code": "111111"}, http.StatusUnauthorized); got != "auth.mfa_challenge_expired" {
		t.Fatalf("expired: %q", got)
	}
	if got := e.client().errorCode(http.MethodPost, "/auth/mfa", map[string]any{"challenge": "nope"}, http.StatusUnauthorized); got != "auth.mfa_challenge_expired" {
		t.Fatalf("unknown: %q", got)
	}
}

func TestMFARequirement(t *testing.T) {
	e := newEnv(t)
	adm := e.admin()
	member := builtinRole(t, adm, "member")
	adm.call(http.MethodPost, "/users", map[string]any{"username": "anna", "display_name": "Anna", "password": testPassword}, http.StatusCreated, nil)
	anna := e.login("anna", testPassword)
	anna.call(http.MethodGet, "/vehicles", nil, http.StatusOK, nil)

	// Requiring it for the role applies on the next request.
	var role struct {
		Role roleJSON `json:"role"`
	}
	adm.call(http.MethodPatch, "/roles/"+itoa(member.ID), map[string]any{"require_mfa": true}, http.StatusOK, &role)
	if !role.Role.RequireMFA {
		t.Fatal("role flag not saved")
	}
	if code := anna.errorCode(http.MethodGet, "/vehicles", nil, http.StatusForbidden); code != "auth.mfa_setup_required" {
		t.Fatalf("code %q", code)
	}
	var me meResp
	anna.call(http.MethodGet, "/auth/me", nil, http.StatusOK, &me)
	if me.User.MFASetupRequired == nil || !*me.User.MFASetupRequired {
		t.Fatalf("me %+v", me.User)
	}

	// Every route except the self-service ones refuses (SC-004).
	for _, p := range e.srv.patterns {
		a := e.srv.routeAccess[p]
		if a.level == levelPublic || a.setup {
			continue
		}
		method, path, _ := strings.Cut(p, " ")
		path = strings.ReplaceAll(strings.TrimPrefix(path, "/api/v1"), "{id}", "1")
		if rec := anna.do(method, path, map[string]any{}); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "auth.mfa_setup_required") {
			t.Errorf("%s during setup: %d %s", p, rec.Code, rec.Body)
		}
	}

	setUpTOTP(t, e, anna)
	anna.call(http.MethodGet, "/vehicles", nil, http.StatusOK, nil)
	if code := anna.errorCode(http.MethodDelete, "/me/totp", nil, http.StatusConflict); code != "mfa.required" {
		t.Fatalf("removing the last factor: %q", code)
	}

	// The admin list shows status and requirement; a requirement per user works too.
	var users struct {
		Items []userJSON `json:"items"`
	}
	adm.call(http.MethodGet, "/users", nil, http.StatusOK, &users)
	i := slices.IndexFunc(users.Items, func(u userJSON) bool { return u.Username == "anna" })
	if !users.Items[i].MFAEnabled || users.Items[i].MFARequired {
		t.Fatalf("anna in list %+v", users.Items[i])
	}
	adm.call(http.MethodPatch, "/roles/"+itoa(member.ID), map[string]any{"require_mfa": false}, http.StatusOK, nil)
	adm.call(http.MethodPatch, "/users/"+itoa(users.Items[i].ID), map[string]any{"mfa_required": true}, http.StatusOK, nil)
	if code := anna.errorCode(http.MethodDelete, "/me/totp", nil, http.StatusConflict); code != "mfa.required" {
		t.Fatalf("user requirement: %q", code)
	}

	// The Administrator role takes only the requirement.
	// The requirement applies to the administrator at once, too.
	adminRole := builtinRole(t, adm, "admin")
	if code := adm.errorCode(http.MethodPatch, "/roles/"+itoa(adminRole.ID), map[string]any{"description": "x"}, http.StatusConflict); code != "role.builtin_locked" {
		t.Fatalf("code %q", code)
	}
	adm.call(http.MethodPatch, "/roles/"+itoa(adminRole.ID), map[string]any{"require_mfa": true}, http.StatusOK, nil)
	if code := adm.errorCode(http.MethodGet, "/users", nil, http.StatusForbidden); code != "auth.mfa_setup_required" {
		t.Fatalf("admin without a factor: %q", code)
	}

	// Resetting anna's factors ends her sessions.
	setUpTOTP(t, e, adm)
	adm.call(http.MethodGet, "/users", nil, http.StatusOK, nil)
	adm.call(http.MethodDelete, "/users/"+itoa(users.Items[i].ID)+"/mfa", nil, http.StatusNoContent, nil)
	anna.call(http.MethodGet, "/auth/me", nil, http.StatusUnauthorized, nil)
	anna = e.login("anna", testPassword)
	if code := anna.errorCode(http.MethodGet, "/vehicles", nil, http.StatusForbidden); code != "auth.mfa_setup_required" {
		t.Fatalf("after reset: %q", code)
	}
}

func TestMFADemoLocked(t *testing.T) {
	cfg, err := config.Load(func(k string) string {
		switch k {
		case "TORQ_DATA_DIR":
			return t.TempDir()
		case "TORQ_DEMO_MODE":
			return "true"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	e := newEnvWithConfig(t, cfg)
	adm := e.admin()
	adm.call(http.MethodGet, "/me/mfa", nil, http.StatusOK, nil)
	for _, r := range []struct{ method, path string }{
		{http.MethodPost, "/me/totp"}, {http.MethodPost, "/me/totp/confirm"}, {http.MethodPost, "/me/recovery-codes"},
		{http.MethodPost, "/me/passkeys/options"}, {http.MethodDelete, "/users/1/mfa"},
	} {
		if code := adm.errorCode(r.method, r.path, map[string]any{"code": "123456"}, http.StatusForbidden); code != "demo.accounts_locked" {
			t.Errorf("%s %s: %q", r.method, r.path, code)
		}
	}
}

// passkeyEnv is a server with TORQ_BASE_URL, where passkeys work.
func passkeyEnv(t *testing.T) *env {
	t.Helper()
	cfg, err := config.Load(func(k string) string {
		switch k {
		case "TORQ_DATA_DIR":
			return t.TempDir()
		case "TORQ_BASE_URL":
			return "https://torq.example.org"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	return newEnvWithConfig(t, cfg)
}

func addPasskey(t *testing.T, c *client, k *softKey) (passkeyJSON, []string) {
	t.Helper()
	var opts struct {
		Session string         `json:"session"`
		Options map[string]any `json:"options"`
	}
	c.call(http.MethodPost, "/me/passkeys/options", nil, http.StatusOK, &opts)
	var out struct {
		Passkey       passkeyJSON `json:"passkey"`
		RecoveryCodes []string    `json:"recovery_codes"`
	}
	c.call(http.MethodPost, "/me/passkeys", map[string]any{"session": opts.Session, "name": " Phone ", "credential": k.create(opts.Options)}, http.StatusCreated, &out)
	return out.Passkey, out.RecoveryCodes
}

func TestPasskeys(t *testing.T) {
	e := passkeyEnv(t)
	adm := e.admin()
	var info map[string]any
	e.client().call(http.MethodGet, "/instance", nil, http.StatusOK, &info)
	if info["passkeys"] != true {
		t.Fatalf("instance %v", info)
	}

	key := newSoftKey(t, "https://torq.example.org", "torq.example.org")
	pk, codes := addPasskey(t, adm, key)
	if pk.Name != "Phone" || !pk.UserVerified || len(codes) != mfa.RecoveryCodeCount {
		t.Fatalf("passkey %+v, %d codes", pk, len(codes))
	}

	// Sign-in without username or password.
	signIn := func(k *softKey, want int) *client {
		var opts struct {
			Session string         `json:"session"`
			Options map[string]any `json:"options"`
		}
		c := e.client()
		c.call(http.MethodPost, "/auth/passkey/options", nil, http.StatusOK, &opts)
		rec := c.do(http.MethodPost, "/auth/passkey", map[string]any{"session": opts.Session, "credential": k.get(opts.Options)})
		if rec.Code != want {
			t.Fatalf("passkey sign-in: %d %s", rec.Code, rec.Body)
		}
		return c
	}
	c := signIn(key, http.StatusOK)
	var me meResp
	c.call(http.MethodGet, "/auth/me", nil, http.StatusOK, &me)
	if me.User.Username != "marek" {
		t.Fatalf("signed in as %q", me.User.Username)
	}

	// Without user verification it can't replace the password, but it can
	// be the second factor after it.
	key.verify = false
	signIn(key, http.StatusUnauthorized)
	id, methods := challenge(t, e, "marek", nil)
	if !slices.Contains(methods, any("passkey")) {
		t.Fatalf("methods %v", methods)
	}
	var opts struct {
		Options map[string]any `json:"options"`
	}
	c = e.client()
	c.call(http.MethodPost, "/auth/mfa/passkey-options", map[string]any{"challenge": id}, http.StatusOK, &opts)
	c.call(http.MethodPost, "/auth/mfa", map[string]any{"challenge": id, "passkey": key.get(opts.Options)}, http.StatusOK, nil)
	c.call(http.MethodGet, "/vehicles", nil, http.StatusOK, nil)

	// A passkey from another server address is refused.
	key.verify = true
	other := newSoftKey(t, "https://evil.example.org", "evil.example.org")
	other.handle = key.handle
	other.id = key.id
	signIn(other, http.StatusUnauthorized)

	// Renamed and removed passkeys.
	adm.call(http.MethodPatch, "/me/passkeys/"+itoa(pk.ID), map[string]any{"name": "Old phone"}, http.StatusOK, nil)
	adm.call(http.MethodDelete, "/me/passkeys/"+itoa(pk.ID), nil, http.StatusNoContent, nil)
	signIn(key, http.StatusUnauthorized)
	var st struct {
		Passkeys          []passkeyJSON `json:"passkeys"`
		RecoveryCodesLeft int64         `json:"recovery_codes_left"`
	}
	adm.call(http.MethodGet, "/me/mfa", nil, http.StatusOK, &st)
	if len(st.Passkeys) != 0 || st.RecoveryCodesLeft != 0 {
		t.Fatalf("after removing the only factor %+v", st)
	}
	e.login("marek", testPassword)
}

func TestPasskeysNeedBaseURL(t *testing.T) {
	e := newEnv(t)
	adm := e.admin()
	if code := adm.errorCode(http.MethodPost, "/me/passkeys/options", nil, http.StatusConflict); code != "passkey.unavailable" {
		t.Fatalf("code %q", code)
	}
	if code := e.client().errorCode(http.MethodPost, "/auth/passkey/options", nil, http.StatusConflict); code != "passkey.unavailable" {
		t.Fatalf("code %q", code)
	}
	var st struct {
		PasskeysAvailable bool `json:"passkeys_available"`
	}
	adm.call(http.MethodGet, "/me/mfa", nil, http.StatusOK, &st)
	if st.PasskeysAvailable {
		t.Fatal("passkeys reported available")
	}
}
