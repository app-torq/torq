package httpapi

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

type sessionList struct {
	Items []struct {
		ID         int64  `json:"id"`
		DeviceName string `json:"device_name"`
		Browser    string `json:"browser"`
		OS         string `json:"os"`
		CreatedAt  string `json:"created_at"`
		LastSeenAt string `json:"last_seen_at"`
		IP         string `json:"ip"`
		Current    bool   `json:"current"`
	} `json:"items"`
	Total int `json:"total"`
}

func TestListSessions(t *testing.T) {
	e := newEnv(t)
	adm := e.admin()
	e.clock.Advance(2 * time.Hour)
	browser := &client{e: e, ip: "198.51.100.7", userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 14.5; rv:132.0) Gecko/20100101 Firefox/132.0"}
	browser.call(http.MethodPost, "/auth/login", map[string]string{"username": "marek", "password": testPassword}, http.StatusOK, nil)
	e.clock.Advance(2 * time.Hour)
	app := e.tokenLogin("marek", testPassword, "  Marek's iPhone\n")
	// Another user's sessions never show up.
	adm.call(http.MethodPost, "/users", map[string]string{
		"username": "partner", "display_name": "Partner", "password": testPassword, "role": "member",
	}, http.StatusCreated, nil)
	e.login("partner", testPassword)

	var out sessionList
	browser.call(http.MethodGet, "/me/sessions", nil, http.StatusOK, &out)
	if out.Total != 3 || len(out.Items) != 3 {
		t.Fatalf("sessions %+v", out)
	}
	for i := 1; i < len(out.Items); i++ {
		if out.Items[i-1].LastSeenAt < out.Items[i].LastSeenAt {
			t.Fatalf("not ordered by last activity: %+v", out.Items)
		}
	}
	current := 0
	for _, s := range out.Items {
		if s.Current {
			current++
			if s.Browser != "Firefox" || s.OS != "macOS" || s.IP != "198.51.100.7" || s.DeviceName != "" || s.CreatedAt == "" {
				t.Fatalf("browser session %+v", s)
			}
		}
		if s.DeviceName == "Marek's iPhone" && s.Current {
			t.Fatal("the app's session is not the browser's current one")
		}
	}
	if current != 1 {
		t.Fatalf("%d sessions marked current", current)
	}
	app.call(http.MethodGet, "/me/sessions", nil, http.StatusOK, &out)
	for _, s := range out.Items {
		if s.Current != (s.DeviceName == "Marek's iPhone") {
			t.Fatalf("the app must see its own session as current: %+v", out.Items)
		}
	}
}

func TestRevokeSessions(t *testing.T) {
	e := newEnv(t)
	browser := e.admin()
	app := e.tokenLogin("marek", testPassword, "phone")
	tablet := e.tokenLogin("marek", testPassword, "tablet")
	browser.call(http.MethodPost, "/users", map[string]string{
		"username": "partner", "display_name": "Partner", "password": testPassword, "role": "member",
	}, http.StatusCreated, nil)
	partner := e.tokenLogin("partner", testPassword, "partner phone")

	var out sessionList
	partner.call(http.MethodGet, "/me/sessions", nil, http.StatusOK, &out)
	partnerID := out.Items[0].ID
	app.call(http.MethodGet, "/me/sessions", nil, http.StatusOK, &out)
	var appID, tabletID int64
	for _, s := range out.Items {
		switch s.DeviceName {
		case "phone":
			appID = s.ID
		case "tablet":
			tabletID = s.ID
		}
	}

	// Someone else's session and unknown IDs look the same: not found.
	for _, id := range []int64{partnerID, 99999} {
		if code := browser.errorCode(http.MethodDelete, fmt.Sprintf("/me/sessions/%d", id), nil, http.StatusNotFound); code != "not_found" {
			t.Fatalf("revoke %d: %q", id, code)
		}
	}
	partner.call(http.MethodGet, "/auth/me", nil, http.StatusOK, nil)

	browser.call(http.MethodDelete, fmt.Sprintf("/me/sessions/%d", tabletID), nil, http.StatusNoContent, nil)
	tablet.errorCode(http.MethodGet, "/auth/me", nil, http.StatusUnauthorized)

	// Revoking the current session signs out; a token client gets no cookie header.
	rec := app.do(http.MethodDelete, fmt.Sprintf("/me/sessions/%d", appID), nil)
	if rec.Code != http.StatusNoContent || len(rec.Result().Cookies()) != 0 {
		t.Fatalf("revoke current token: %d %v", rec.Code, rec.Result().Cookies())
	}
	app.errorCode(http.MethodGet, "/auth/me", nil, http.StatusUnauthorized)

	// "Sign out all other devices" keeps only the caller.
	other := e.tokenLogin("marek", testPassword, "laptop")
	browser.call(http.MethodDelete, "/me/sessions", nil, http.StatusNoContent, nil)
	other.errorCode(http.MethodGet, "/auth/me", nil, http.StatusUnauthorized)
	browser.call(http.MethodGet, "/me/sessions", nil, http.StatusOK, &out)
	if out.Total != 1 || !out.Items[0].Current {
		t.Fatalf("after revoking others: %+v", out)
	}
	partner.call(http.MethodGet, "/auth/me", nil, http.StatusOK, nil)

	// Revoking the current cookie session clears the cookie.
	rec = browser.do(http.MethodDelete, fmt.Sprintf("/me/sessions/%d", out.Items[0].ID), nil)
	if rec.Code != http.StatusNoContent || len(rec.Result().Cookies()) != 1 || rec.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatalf("revoke current cookie: %d %v", rec.Code, rec.Result().Cookies())
	}
}

func TestSessionsLockedInDemo(t *testing.T) {
	cfg := newEnv(t).cfg
	cfg.DemoMode = true
	e := newEnvWithConfig(t, cfg)
	c := e.admin()
	for _, r := range []struct{ method, path string }{
		{http.MethodGet, "/me/sessions"}, {http.MethodDelete, "/me/sessions/1"}, {http.MethodDelete, "/me/sessions"},
	} {
		if code := c.errorCode(r.method, r.path, nil, http.StatusForbidden); code != "demo.accounts_locked" {
			t.Errorf("%s %s: %q", r.method, r.path, code)
		}
	}
}
