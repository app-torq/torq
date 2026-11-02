package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/app-torq/torq/internal/config"
	"github.com/app-torq/torq/internal/testutil"
)

// Every mutating route must reject requests without the CSRF header, even
// with a valid session cookie.
func TestCSRFOnAllMutatingRoutes(t *testing.T) {
	cfg, _ := config.Load(func(k string) string {
		if k == "TORQ_DATA_DIR" {
			return t.TempDir()
		}
		return ""
	})
	s := NewServer(Deps{
		Config: cfg, DB: testutil.NewDB(t), Clock: testutil.FixedClock(t, "2026-10-30"),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Version: "test", Dist: fstest.MapFS{"index.html": {}},
	})
	h := s.Handler()
	e := &env{t: t, h: h}
	adm := e.admin()

	checked := 0
	for _, p := range s.patterns {
		method, path, _ := strings.Cut(p, " ")
		if method == http.MethodGet || method == http.MethodHead {
			continue
		}
		path = strings.NewReplacer("{id}", "1").Replace(path)
		req := httptest.NewRequest(method, path, strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(adm.cookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "csrf.missing_header") {
			t.Errorf("%s: got %d %s", p, rec.Code, rec.Body)
		}
		checked++
	}
	if checked < 50 {
		t.Fatalf("only %d mutating routes checked", checked)
	}
}

func TestSessionCookieFlags(t *testing.T) {
	cfg, _ := config.Load(func(k string) string {
		switch k {
		case "TORQ_DATA_DIR":
			return t.TempDir()
		case "TORQ_BASE_URL":
			return "https://torq.example"
		}
		return ""
	})
	e := newEnvWithConfig(t, cfg)
	rec := e.client().do(http.MethodPost, "/setup", map[string]string{"username": "marek", "display_name": "M", "password": testPassword})
	c := rec.Result().Cookies()[0]
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Fatalf("cookie flags %+v", c)
	}
}
