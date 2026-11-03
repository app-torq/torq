package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/app-torq/torq/internal/config"
)

func TestSetupFlow(t *testing.T) {
	e := newEnv(t)
	c := e.client()
	var status struct {
		SetupRequired bool `json:"setup_required"`
	}
	c.call(http.MethodGet, "/setup/status", nil, http.StatusOK, &status)
	if !status.SetupRequired {
		t.Fatal("fresh instance must require setup")
	}
	if code := c.errorCode(http.MethodGet, "/auth/me", nil, http.StatusUnauthorized); code != "auth.unauthenticated" {
		t.Fatalf("code %q", code)
	}

	admin := e.admin()
	var me struct {
		User struct {
			Role string `json:"role"`
		} `json:"user"`
		Instance struct {
			MaxUploadMB int `json:"max_upload_mb"`
		} `json:"instance"`
	}
	admin.call(http.MethodGet, "/auth/me", nil, http.StatusOK, &me)
	if me.User.Role != "admin" || me.Instance.MaxUploadMB != 25 {
		t.Fatalf("me = %+v", me)
	}

	c.call(http.MethodGet, "/setup/status", nil, http.StatusOK, &status)
	if status.SetupRequired {
		t.Fatal("setup must not be required after setup")
	}
	code := c.errorCode(http.MethodPost, "/setup", map[string]string{
		"username": "intruder", "display_name": "X", "password": testPassword,
	}, http.StatusConflict)
	if code != "setup.already_completed" {
		t.Fatalf("code %q", code)
	}
}

func TestSetupValidation(t *testing.T) {
	e := newEnv(t)
	var out struct {
		Error struct {
			Code   string            `json:"code"`
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	e.client().call(http.MethodPost, "/setup", map[string]string{
		"username": "a", "display_name": "", "password": "short",
	}, http.StatusUnprocessableEntity, &out)
	if out.Error.Fields["username"] != "validation.username_format" ||
		out.Error.Fields["password"] != "validation.password_too_short" ||
		out.Error.Fields["display_name"] != "validation.length" {
		t.Fatalf("fields %v", out.Error.Fields)
	}
}

func TestLoginErrorsAreIndistinguishable(t *testing.T) {
	e := newEnv(t)
	e.admin()
	c := e.client()
	wrong := c.do(http.MethodPost, "/auth/login", map[string]string{"username": "marek", "password": "nope-nope-nope"})
	unknown := c.do(http.MethodPost, "/auth/login", map[string]string{"username": "ghost", "password": "nope-nope-nope"})
	if wrong.Code != http.StatusUnauthorized || unknown.Code != http.StatusUnauthorized {
		t.Fatalf("codes %d %d", wrong.Code, unknown.Code)
	}
	if wrong.Body.String() != unknown.Body.String() {
		t.Fatalf("bodies differ: %s vs %s", wrong.Body, unknown.Body)
	}
	// Usernames are case-insensitive.
	e.login("MAREK", testPassword)
}

func TestLoginThrottle(t *testing.T) {
	e := newEnv(t)
	e.admin()
	c := e.client()
	for i := 0; i < 5; i++ {
		c.errorCode(http.MethodPost, "/auth/login", map[string]string{"username": "marek", "password": "wrong-password"}, http.StatusUnauthorized)
	}
	var out struct {
		Error struct {
			Code   string         `json:"code"`
			Params map[string]any `json:"params"`
		} `json:"error"`
	}
	c.call(http.MethodPost, "/auth/login", map[string]string{"username": "marek", "password": testPassword}, http.StatusTooManyRequests, &out)
	if out.Error.Code != "auth.throttled" || out.Error.Params["retry_after_s"] == nil {
		t.Fatalf("got %+v", out.Error)
	}
	e.clock.Advance(2 * time.Second)
	e.login("marek", testPassword)
}

func TestLoginThrottleDoesNotLockOutOtherIPs(t *testing.T) {
	e := newEnv(t)
	e.admin()
	attacker := e.clientFrom("198.51.100.10")
	for i := 0; i < 5; i++ {
		attacker.errorCode(http.MethodPost, "/auth/login", map[string]string{"username": "marek", "password": "wrong-password"}, http.StatusUnauthorized)
	}
	attacker.errorCode(http.MethodPost, "/auth/login", map[string]string{"username": "marek", "password": testPassword}, http.StatusTooManyRequests)
	e.clientFrom("203.0.113.5").call(http.MethodPost, "/auth/login", map[string]string{"username": "marek", "password": testPassword}, http.StatusOK, nil)
}

func TestLoginSuccessKeepsIPFailures(t *testing.T) {
	e := newEnv(t)
	e.admin().call(http.MethodPost, "/users", map[string]string{
		"username": "partner", "display_name": "Partner", "password": testPassword, "role": "member",
	}, http.StatusCreated, nil)
	c := e.clientFrom("198.51.100.20")
	for i := 0; i < 4; i++ {
		c.errorCode(http.MethodPost, "/auth/login", map[string]string{"username": "marek", "password": "wrong-password"}, http.StatusUnauthorized)
	}
	c.call(http.MethodPost, "/auth/login", map[string]string{"username": "partner", "password": testPassword}, http.StatusOK, nil)
	// The fifth failure from this IP still blocks it.
	c.errorCode(http.MethodPost, "/auth/login", map[string]string{"username": "ghost", "password": "wrong-password"}, http.StatusUnauthorized)
	c.errorCode(http.MethodPost, "/auth/login", map[string]string{"username": "other", "password": "wrong-password"}, http.StatusTooManyRequests)
}

func TestLoginThrottlesDistributedGuessing(t *testing.T) {
	e := newEnv(t)
	e.admin()
	for i := 0; i < 50; i++ {
		e.clientFrom(fmt.Sprintf("198.51.100.%d", i)).errorCode(http.MethodPost, "/auth/login",
			map[string]string{"username": "marek", "password": "wrong-password"}, http.StatusUnauthorized)
	}
	e.clientFrom("203.0.113.5").errorCode(http.MethodPost, "/auth/login", map[string]string{"username": "marek", "password": testPassword}, http.StatusTooManyRequests)
}

func TestLoginRejectsOverlongCredentials(t *testing.T) {
	e := newEnv(t)
	e.admin()
	long := strings.Repeat("a", 64<<10)
	c := e.clientFrom("198.51.100.30")
	for _, in := range []map[string]string{
		{"username": long, "password": testPassword},
		{"username": "marek", "password": long},
	} {
		if code := c.errorCode(http.MethodPost, "/auth/login", in, http.StatusUnauthorized); code != "auth.invalid_credentials" {
			t.Fatalf("code %q", code)
		}
	}
	// They count against the IP but not the user.
	if e.srv.throttle.RetryAfter("user:marek@ip:198.51.100.30") != 0 || e.srv.userThrottle.RetryAfter("user:marek") != 0 {
		t.Fatal("user keys must not be throttled")
	}
	var out struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	e.login("marek", testPassword).call(http.MethodPost, "/auth/password", map[string]string{
		"current_password": testPassword, "new_password": long,
	}, http.StatusUnprocessableEntity, &out)
	if out.Error.Fields["new_password"] != "validation.password_too_long" {
		t.Fatalf("fields %v", out.Error.Fields)
	}
}

func TestSessionCookieSecureOverHTTPS(t *testing.T) {
	secure := func(mode config.CookieSecure, remote, proto string) bool {
		t.Helper()
		cfg := newEnv(t).cfg
		cfg.CookieSecure = mode
		cfg.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
		e := newEnvWithConfig(t, cfg)
		e.admin()
		body, _ := json.Marshal(map[string]string{"username": "marek", "password": testPassword})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Torq-Request", "1")
		req.Header.Set("X-Forwarded-Proto", proto)
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		e.h.ServeHTTP(rec, req)
		cookies := rec.Result().Cookies()
		if rec.Code != http.StatusOK || len(cookies) != 1 {
			t.Fatalf("login: %d %v", rec.Code, cookies)
		}
		return cookies[0].Secure
	}
	if !secure(config.CookieSecureAuto, "10.0.0.2:1234", "https") {
		t.Error("auto behind a trusted HTTPS proxy must be Secure")
	}
	if secure(config.CookieSecureAuto, "203.0.113.5:1234", "https") {
		t.Error("an untrusted peer's X-Forwarded-Proto must be ignored")
	}
	if secure(config.CookieSecureFalse, "10.0.0.2:1234", "https") {
		t.Error("TORQ_COOKIE_SECURE=false must win")
	}
}

func TestTokenLogin(t *testing.T) {
	e := newEnv(t)
	e.admin()
	rec := e.client().do(http.MethodPost, "/auth/login", map[string]any{
		"username": "marek", "password": testPassword, "token": true, "device_name": "Marek's iPhone",
	})
	var out struct {
		Token    string         `json:"token"`
		User     userJSON       `json:"user"`
		Instance map[string]any `json:"instance"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	if out.Token == "" || out.User.Username != "marek" || out.Instance == nil {
		t.Fatalf("response %s", rec.Body)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatalf("token login must not set a cookie: %v", rec.Result().Cookies())
	}

	// Without "token" the response is unchanged: a cookie and no token field.
	rec = e.client().do(http.MethodPost, "/auth/login", map[string]string{"username": "marek", "password": testPassword})
	if len(rec.Result().Cookies()) != 1 || strings.Contains(rec.Body.String(), `"token"`) {
		t.Fatalf("cookie login changed: %v %s", rec.Result().Cookies(), rec.Body)
	}

	// Bearer requests need no X-Torq-Request header.
	app := &client{e: e, token: out.Token}
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/me/preferences", strings.NewReader(`{"locale":"de"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+out.Token)
	rr := httptest.NewRecorder()
	e.h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"locale":"de"`) {
		t.Fatalf("bearer PATCH: %d %s", rr.Code, rr.Body)
	}
	app.call(http.MethodGet, "/auth/me", nil, http.StatusOK, nil)
}

func TestTokenSetup(t *testing.T) {
	e := newEnv(t)
	rec := e.client().do(http.MethodPost, "/setup", map[string]any{
		"username": "marek", "display_name": "Marek", "password": testPassword, "token": true,
	})
	if rec.Code != http.StatusCreated || len(rec.Result().Cookies()) != 0 || !strings.Contains(rec.Body.String(), `"token":"`) {
		t.Fatalf("setup: %d %v %s", rec.Code, rec.Result().Cookies(), rec.Body)
	}
}

func TestBearerRejections(t *testing.T) {
	e := newEnv(t)
	browser := e.admin()
	app := e.tokenLogin("marek", testPassword, "phone")
	me := func(authz string, cookie bool) int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", authz)
		if cookie {
			req.AddCookie(browser.cookie)
		}
		rec := httptest.NewRecorder()
		e.h.ServeHTTP(rec, req)
		if rec.Code == http.StatusUnauthorized && !strings.Contains(rec.Body.String(), "auth.unauthenticated") {
			t.Errorf("%q: body %s", authz, rec.Body)
		}
		return rec.Code
	}
	// An invalid bearer token is rejected even with a valid cookie (FR-004).
	for _, h := range []string{"Bearer nonsense", "Bearer ", "Bearer", "bearer " + app.token + "x"} {
		if code := me(h, true); code != http.StatusUnauthorized {
			t.Errorf("%q with cookie: got %d", h, code)
		}
	}
	// Other schemes belong to a reverse proxy (e.g. basic auth) and are ignored.
	for _, h := range []string{"Basic bWFyZWs6eA==", "garbage"} {
		if code := me(h, true); code != http.StatusOK {
			t.Errorf("%q with cookie: got %d", h, code)
		}
		if code := me(h, false); code != http.StatusUnauthorized {
			t.Errorf("%q without cookie: got %d", h, code)
		}
	}
	// The scheme is case-insensitive.
	if code := me("bearer "+app.token, false); code != http.StatusOK {
		t.Errorf("lowercase scheme: got %d", code)
	}
}

func TestTokenLogoutRevokesOnlyThatToken(t *testing.T) {
	e := newEnv(t)
	browser := e.admin()
	first := e.tokenLogin("marek", testPassword, "phone")
	second := e.tokenLogin("marek", testPassword, "tablet")
	rec := first.do(http.MethodPost, "/auth/logout", nil)
	if rec.Code != http.StatusNoContent || len(rec.Result().Cookies()) != 0 {
		t.Fatalf("logout: %d %v", rec.Code, rec.Result().Cookies())
	}
	first.errorCode(http.MethodGet, "/auth/me", nil, http.StatusUnauthorized)
	second.call(http.MethodGet, "/auth/me", nil, http.StatusOK, nil)
	browser.call(http.MethodGet, "/auth/me", nil, http.StatusOK, nil)

	// A password change revokes every other sign-in, tokens included.
	browser.call(http.MethodPost, "/auth/password", map[string]string{
		"current_password": testPassword, "new_password": "a-new-password-1",
	}, http.StatusNoContent, nil)
	second.errorCode(http.MethodGet, "/auth/me", nil, http.StatusUnauthorized)
	browser.call(http.MethodGet, "/auth/me", nil, http.StatusOK, nil)
}

func TestTokenLoginErrorsMatchCookieLogin(t *testing.T) {
	e := newEnv(t)
	e.admin()
	c := e.clientFrom("198.51.100.40")
	cookie := c.do(http.MethodPost, "/auth/login", map[string]any{"username": "marek", "password": "wrong-password"})
	token := c.do(http.MethodPost, "/auth/login", map[string]any{"username": "marek", "password": "wrong-password", "token": true})
	if cookie.Code != http.StatusUnauthorized || token.Body.String() != cookie.Body.String() {
		t.Fatalf("bodies differ: %s vs %s", cookie.Body, token.Body)
	}
	// Both count against the same throttle: three more failures block.
	for i := 0; i < 3; i++ {
		c.do(http.MethodPost, "/auth/login", map[string]any{"username": "marek", "password": "wrong-password", "token": true})
	}
	c.errorCode(http.MethodPost, "/auth/login", map[string]any{"username": "marek", "password": testPassword, "token": true}, http.StatusTooManyRequests)
}

func TestTokenExpirySlides(t *testing.T) {
	e := newEnv(t)
	e.admin()
	app := e.tokenLogin("marek", testPassword, "phone")
	e.clock.Advance(20 * 24 * time.Hour)
	app.call(http.MethodGet, "/auth/me", nil, http.StatusOK, nil)
	e.clock.Advance(20 * 24 * time.Hour)
	app.call(http.MethodGet, "/auth/me", nil, http.StatusOK, nil)
	e.clock.Advance(31 * 24 * time.Hour)
	app.errorCode(http.MethodGet, "/auth/me", nil, http.StatusUnauthorized)
}

func TestInstance(t *testing.T) {
	e := newEnv(t)
	var info map[string]any
	e.client().call(http.MethodGet, "/instance", nil, http.StatusOK, &info)
	want := map[string]any{
		"api_version": float64(APIVersion), "version": "test", "setup_required": true,
		"demo_mode": false, "token_auth": true, "spec_autofill": false, "max_upload_mb": float64(25),
	}
	for k, v := range want {
		if info[k] != v {
			t.Errorf("%s = %v, want %v", k, info[k], v)
		}
	}
	if len(info) != len(want) {
		t.Errorf("unexpected fields: %v", info)
	}

	adm := e.admin()
	e.client().call(http.MethodGet, "/instance", nil, http.StatusOK, &info)
	if info["setup_required"] != false {
		t.Fatal("setup_required after setup")
	}
	// /auth/me reports the same values (FR-017).
	var me struct {
		Instance map[string]any `json:"instance"`
	}
	adm.call(http.MethodGet, "/auth/me", nil, http.StatusOK, &me)
	for k, v := range info {
		if k != "setup_required" && me.Instance[k] != v {
			t.Errorf("me.instance.%s = %v, /instance has %v", k, me.Instance[k], v)
		}
	}
}

func TestInstanceInDemoMode(t *testing.T) {
	cfg := newEnv(t).cfg
	cfg.DemoMode, cfg.MaxUploadMB = true, 0
	e := newEnvWithConfig(t, cfg)
	var info map[string]any
	e.client().call(http.MethodGet, "/instance", nil, http.StatusOK, &info)
	if info["demo_mode"] != true || info["max_upload_mb"] != float64(0) {
		t.Fatalf("instance %v", info)
	}
}

func TestLogoutAndPasswordChange(t *testing.T) {
	e := newEnv(t)
	first := e.admin()
	second := e.login("marek", testPassword)

	if code := first.errorCode(http.MethodPost, "/auth/password", map[string]string{
		"current_password": "not-the-password", "new_password": "a-new-password-1",
	}, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("code %q", code)
	}
	first.call(http.MethodPost, "/auth/password", map[string]string{
		"current_password": testPassword, "new_password": "a-new-password-1",
	}, http.StatusNoContent, nil)

	// The other session is revoked; the current one survives.
	second.errorCode(http.MethodGet, "/auth/me", nil, http.StatusUnauthorized)
	first.call(http.MethodGet, "/auth/me", nil, http.StatusOK, nil)
	e.login("marek", "a-new-password-1")

	first.call(http.MethodPost, "/auth/logout", nil, http.StatusNoContent, nil)
	first.errorCode(http.MethodGet, "/auth/me", nil, http.StatusUnauthorized)
}

func TestPreferences(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	var out struct {
		User userJSON `json:"user"`
	}
	c.call(http.MethodPatch, "/me/preferences", map[string]any{
		"locale": "de", "date_format": "DD.MM.YYYY", "number_format": "comma", "currency_code": "chf", "currency_symbol": "Fr.",
	}, http.StatusOK, &out)
	if out.User.Locale != "de" || out.User.CurrencyCode != "CHF" || out.User.CurrencySymbol == nil || *out.User.CurrencySymbol != "Fr." {
		t.Fatalf("user %+v", out.User)
	}
	c.call(http.MethodPatch, "/me/preferences", map[string]any{"currency_symbol": ""}, http.StatusOK, &out)
	if out.User.CurrencySymbol != nil {
		t.Fatal("empty symbol must clear the override")
	}
	if code := c.errorCode(http.MethodPatch, "/me/preferences", map[string]any{"locale": "fr"}, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("code %q", code)
	}
}

func TestUserAdministration(t *testing.T) {
	e := newEnv(t)
	adm := e.admin()
	var created struct {
		User userJSON `json:"user"`
	}
	adm.call(http.MethodPost, "/users", map[string]string{
		"username": "partner", "display_name": "Partner", "password": testPassword, "role": "member",
	}, http.StatusCreated, &created)

	member := e.login("partner", testPassword)
	if code := member.errorCode(http.MethodGet, "/users", nil, http.StatusForbidden); code != "auth.forbidden" {
		t.Fatalf("code %q", code)
	}

	var me struct {
		User userJSON `json:"user"`
	}
	adm.call(http.MethodGet, "/auth/me", nil, http.StatusOK, &me)

	// Demoting the only admin is refused.
	if code := adm.errorCode(http.MethodPatch, "/users/"+itoa(me.User.ID), map[string]string{"role": "member"}, http.StatusConflict); code != "user.last_admin" {
		t.Fatalf("code %q", code)
	}
	if code := adm.errorCode(http.MethodDelete, "/users/"+itoa(me.User.ID), nil, http.StatusConflict); code != "user.cannot_delete_self" {
		t.Fatalf("code %q", code)
	}
	if code := adm.errorCode(http.MethodPost, "/users", map[string]string{
		"username": "PARTNER", "display_name": "Dup", "password": testPassword, "role": "member",
	}, http.StatusConflict); code != "user.username_taken" {
		t.Fatalf("code %q", code)
	}

	// Admin password reset ends the member's sessions.
	adm.call(http.MethodPost, "/users/"+itoa(created.User.ID)+"/password", map[string]string{"new_password": "reset-password-9"}, http.StatusNoContent, nil)
	member.errorCode(http.MethodGet, "/auth/me", nil, http.StatusUnauthorized)

	// Disabling blocks login.
	adm.call(http.MethodPatch, "/users/"+itoa(created.User.ID), map[string]bool{"disabled": true}, http.StatusOK, nil)
	e.client().errorCode(http.MethodPost, "/auth/login", map[string]string{"username": "partner", "password": "reset-password-9"}, http.StatusUnauthorized)

	adm.call(http.MethodDelete, "/users/"+itoa(created.User.ID), nil, http.StatusNoContent, nil)
}
