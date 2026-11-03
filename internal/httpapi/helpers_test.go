package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/config"
	"github.com/app-torq/torq/internal/db"
	"github.com/app-torq/torq/internal/testutil"
)

// env is one test server; clients created from it share the handler and DB.
type env struct {
	t     *testing.T
	h     http.Handler
	srv   *Server
	db    *db.DB
	clock *clock.Fixed
	cfg   config.Config
}

func newEnv(t *testing.T) *env {
	t.Helper()
	cfg, err := config.Load(func(k string) string {
		if k == "TORQ_DATA_DIR" {
			return t.TempDir()
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	return newEnvWithConfig(t, cfg)
}

func newEnvWithConfig(t *testing.T, cfg config.Config) *env {
	t.Helper()
	d := testutil.NewDB(t)
	clk := testutil.FixedClock(t, "2026-10-30")
	srv := NewServer(Deps{
		Config:  cfg,
		DB:      d,
		Clock:   clk,
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Version: "test",
		Dist:    fstest.MapFS{"index.html": {Data: []byte("<html>torq</html>")}, "assets/app.js": {Data: []byte("x")}},
	})
	return &env{t: t, h: srv.Handler(), srv: srv, db: d, clock: clk, cfg: cfg}
}

func newTestRouter(t *testing.T) http.Handler { return newEnv(t).h }

// client is a browser-like session against env.
type client struct {
	e      *env
	cookie *http.Cookie
	ip     string
	// token, when set, is sent as a bearer token instead of the cookie.
	token string
}

func (e *env) client() *client { return &client{e: e} }

// clientFrom returns a client whose requests come from ip.
func (e *env) clientFrom(ip string) *client { return &client{e: e, ip: ip} }

func (c *client) do(method, path string, body any) *httptest.ResponseRecorder {
	c.e.t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			c.e.t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, "/api/v1"+path, r)
	req.Header.Set("Content-Type", "application/json")
	if c.ip != "" {
		req.RemoteAddr = c.ip + ":1234"
	}
	if method != http.MethodGet {
		req.Header.Set("X-Torq-Request", "1")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	} else if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	rec := httptest.NewRecorder()
	c.e.h.ServeHTTP(rec, req)
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == sessionCookie {
			if ck.MaxAge < 0 {
				c.cookie = nil
			} else {
				c.cookie = ck
			}
		}
	}
	return rec
}

// call performs a request, asserts the status, and decodes the body into out (if non-nil).
func (c *client) call(method, path string, body any, wantStatus int, out any) {
	c.e.t.Helper()
	rec := c.do(method, path, body)
	if rec.Code != wantStatus {
		c.e.t.Fatalf("%s %s: status %d, want %d; body %s", method, path, rec.Code, wantStatus, rec.Body)
	}
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			c.e.t.Fatalf("%s %s: decode: %v; body %s", method, path, err, rec.Body)
		}
	}
}

// errorCode performs a request, asserts the status, and returns the error code.
func (c *client) errorCode(method, path string, body any, wantStatus int) string {
	c.e.t.Helper()
	var out struct {
		Error struct {
			Code   string            `json:"code"`
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	c.call(method, path, body, wantStatus, &out)
	return out.Error.Code
}

const testPassword = "workshop-manual-1"

// admin returns a client signed in as the initial administrator.
func (e *env) admin() *client {
	c := e.client()
	c.call(http.MethodPost, "/setup", map[string]string{
		"username": "marek", "display_name": "Marek", "password": testPassword,
	}, http.StatusCreated, nil)
	return c
}

// tokenLogin returns a new client signed in as username with a bearer token.
func (e *env) tokenLogin(username, password, device string) *client {
	e.t.Helper()
	var out struct {
		Token string `json:"token"`
	}
	e.client().call(http.MethodPost, "/auth/login", map[string]any{
		"username": username, "password": password, "token": true, "device_name": device,
	}, http.StatusOK, &out)
	if out.Token == "" {
		e.t.Fatal("login with token: no token in response")
	}
	return &client{e: e, token: out.Token}
}

// login returns a new client signed in as username.
func (e *env) login(username, password string) *client {
	c := e.client()
	c.call(http.MethodPost, "/auth/login", map[string]string{"username": username, "password": password}, http.StatusOK, nil)
	return c
}
