package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthz(t *testing.T) {
	h := newTestRouter(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["status"] != "ok" {
		t.Fatalf("body %s", rec.Body)
	}
}

func TestUnknownAPIRoute(t *testing.T) {
	h := newTestRouter(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"not_found"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestCSRFHeaderRequired(t *testing.T) {
	h := newTestRouter(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/x", strings.NewReader("{}")))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("got %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/x", strings.NewReader("{}"))
	req.Header.Set("X-Torq-Request", "1")
	req.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "csrf.origin_mismatch") {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
}

func TestSPAFallback(t *testing.T) {
	h := newTestRouter(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/vehicles/3/specs", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<html>torq</html>") {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if rec.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("asset cache header %q", rec.Header().Get("Cache-Control"))
	}
}
