package httpapi

import (
	"net/http"
	"testing"
	"time"
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
