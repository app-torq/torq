package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/app-torq/torq/internal/testutil"
)

// The PWA's static files need exact content types and cache headers
// (specs/004-installable-pwa, contracts/browser-contract.md).
func TestSPAHeaders(t *testing.T) {
	dist := fstest.MapFS{
		"index.html":           {Data: []byte("<html>torq</html>")},
		"sw.js":                {Data: []byte("self.x = 1")},
		"manifest.webmanifest": {Data: []byte(`{"name":"torq"}`)},
		"icons/icon-192.png":   {Data: []byte("\x89PNG\r\n\x1a\n")},
		"apple-touch-icon.png": {Data: []byte("\x89PNG\r\n\x1a\n")},
		"assets/app.js":        {Data: []byte("x")},
	}
	h := spaHandler(dist)
	cases := []struct{ path, contentType, cacheControl string }{
		{"/manifest.webmanifest", "application/manifest+json", "no-cache"},
		{"/sw.js", "text/javascript; charset=utf-8", "no-cache"},
		{"/icons/icon-192.png", "image/png", "public, max-age=86400"},
		{"/apple-touch-icon.png", "image/png", "public, max-age=86400"},
		{"/assets/app.js", "", "public, max-age=31536000, immutable"},
		{"/vehicles/7", "text/html; charset=utf-8", "no-cache"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d", c.path, rec.Code)
		}
		if c.contentType != "" && rec.Header().Get("Content-Type") != c.contentType {
			t.Errorf("%s: Content-Type %q, want %q", c.path, rec.Header().Get("Content-Type"), c.contentType)
		}
		if rec.Header().Get("Cache-Control") != c.cacheControl {
			t.Errorf("%s: Cache-Control %q, want %q", c.path, rec.Header().Get("Cache-Control"), c.cacheControl)
		}
	}
}

func TestPWAFilesKeepSecurityHeaders(t *testing.T) {
	h := NewServer(Deps{
		Config: newEnv(t).cfg, DB: testutil.NewDB(t), Clock: testutil.FixedClock(t, "2026-10-30"),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Version: "test",
		Dist: fstest.MapFS{
			"index.html":           {Data: []byte("<html>torq</html>")},
			"sw.js":                {Data: []byte("self.x = 1")},
			"manifest.webmanifest": {Data: []byte(`{"name":"torq"}`)},
		},
	}).Handler()
	for _, p := range []string{"/sw.js", "/manifest.webmanifest"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Security-Policy") == "" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: security headers missing: %v", p, rec.Header())
		}
	}
}
