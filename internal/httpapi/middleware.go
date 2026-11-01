package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// Recover converts panics into 500 responses.
func Recover(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				log.ErrorContext(r.Context(), "panic", "method", r.Method, "path", r.URL.Path, "panic", v)
				writeJSON(w, http.StatusInternalServerError, errorBody{errorPayload{Code: "internal"}})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RequestLog logs method, path, status and duration. Query strings and headers
// are never logged (they may carry secrets).
func RequestLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		var b [6]byte
		_, _ = rand.Read(b[:])
		reqID := hex.EncodeToString(b[:])
		next.ServeHTTP(rec, r)
		level := slog.LevelDebug
		if strings.HasPrefix(r.URL.Path, "/api/") {
			level = slog.LevelInfo
		}
		if rec.status >= 500 {
			level = slog.LevelError
		}
		log.Log(r.Context(), level, "request",
			"id", reqID, "method", r.Method, "path", r.URL.Path,
			"status", rec.status, "duration_ms", time.Since(start).Milliseconds())
	})
}

// CSRF requires X-Torq-Request: 1 on mutating API requests and rejects
// cross-origin Origin headers (research R6).
func CSRF(baseURL string, next http.Handler) http.Handler {
	var baseHost string
	if u, err := url.Parse(baseURL); err == nil {
		baseHost = u.Host
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Method != http.MethodGet && r.Method != http.MethodHead {
			if r.Header.Get("X-Torq-Request") != "1" {
				writeJSON(w, http.StatusForbidden, errorBody{errorPayload{Code: "csrf.missing_header"}})
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || (u.Host != r.Host && (baseHost == "" || u.Host != baseHost)) {
					writeJSON(w, http.StatusForbidden, errorBody{errorPayload{Code: "csrf.origin_mismatch"}})
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders sets conservative defaults on every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "DENY")
		if h.Get("Content-Security-Policy") == "" {
			h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob: data:; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'")
		}
		next.ServeHTTP(w, r)
	})
}

// handlerFunc is an HTTP handler that may return an error to be rendered.
type handlerFunc func(w http.ResponseWriter, r *http.Request) error
