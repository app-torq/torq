package httpapi

import (
	"net/http"
	"testing"
)

func TestDemoModeLocksAccountsAndWebhooks(t *testing.T) {
	e := newEnv(t)
	adm := e.admin()
	adm.call(http.MethodPost, "/users", map[string]string{
		"username": "lena", "display_name": "Lena", "password": testPassword, "role": "member",
	}, http.StatusCreated, nil)
	var users struct {
		Users []struct {
			ID       int64  `json:"id"`
			Username string `json:"username"`
		} `json:"users"`
	}
	adm.call(http.MethodGet, "/users", nil, http.StatusOK, &users)
	var lena int64
	for _, u := range users.Users {
		if u.Username == "lena" {
			lena = u.ID
		}
	}

	hook := map[string]any{"name": "ntfy", "url": "https://ntfy.example/torq", "preset": "ntfy"}
	adm.call(http.MethodPost, "/webhooks", hook, http.StatusCreated, nil)
	var hooks listResponse[webhookJSON]
	adm.call(http.MethodGet, "/webhooks", nil, http.StatusOK, &hooks)
	hookPath := "/webhooks/" + itoa(hooks.Items[0].ID)

	e.srv.Config.DemoMode = true
	locked := []struct {
		method, path string
		body         any
		code         string
	}{
		{http.MethodPost, "/auth/password", map[string]string{"current_password": testPassword, "new_password": "something-else-1"}, accountsLocked},
		{http.MethodPost, "/users", map[string]string{"username": "eve", "display_name": "Eve", "password": testPassword, "role": "admin"}, accountsLocked},
		{http.MethodPatch, "/users/" + itoa(lena), map[string]string{"role": "admin"}, accountsLocked},
		{http.MethodPost, "/users/" + itoa(lena) + "/password", map[string]string{"new_password": "something-else-1"}, accountsLocked},
		{http.MethodDelete, "/users/" + itoa(lena), nil, accountsLocked},
		{http.MethodPost, "/webhooks", hook, webhooksLocked},
		{http.MethodPatch, hookPath, map[string]any{"url": "http://169.254.169.254/"}, webhooksLocked},
		{http.MethodPost, hookPath + "/test", nil, webhooksLocked},
		{http.MethodDelete, hookPath, nil, webhooksLocked},
	}
	for _, c := range locked {
		if code := adm.errorCode(c.method, c.path, c.body, http.StatusForbidden); code != c.code {
			t.Errorf("%s %s: %q", c.method, c.path, code)
		}
	}

	// The login still works and everything else stays usable.
	e.login("marek", testPassword).call(http.MethodGet, "/users", nil, http.StatusOK, nil)
	adm.call(http.MethodGet, "/webhooks", nil, http.StatusOK, nil)
	adm.call(http.MethodPost, "/vehicles", map[string]any{"make": "BMW", "model": "320d", "current_odometer_km": 100000}, http.StatusCreated, nil)
}
