package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/marekbeckmann/torq/internal/apperr"
)

const maxJSONBody = 1 << 20

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code   string            `json:"code"`
	Fields map[string]string `json:"fields,omitempty"`
	Params map[string]any    `json:"params,omitempty"`
}

// writeJSON writes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// writeError renders err in the contract error format. Unknown errors become
// 500 "internal" and are logged.
func writeError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	if e, ok := apperr.As(err); ok {
		if e.Status == http.StatusTooManyRequests {
			if s, ok := e.Params["retry_after_s"].(int); ok {
				w.Header().Set("Retry-After", strconv.Itoa(s))
			}
		}
		writeJSON(w, e.Status, errorBody{errorPayload{Code: e.Code, Fields: e.Fields, Params: e.Params}})
		return
	}
	log.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	writeJSON(w, http.StatusInternalServerError, errorBody{errorPayload{Code: "internal"}})
}

// decodeJSON decodes the request body into dst, rejecting unknown fields and
// bodies over 1 MiB.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return &apperr.Error{Status: http.StatusRequestEntityTooLarge, Code: "request.too_large"}
		}
		return apperr.BadRequest("request.malformed")
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperr.BadRequest("request.malformed")
	}
	return nil
}

// errNotFound is returned for unknown IDs and routes.
var errNotFound = apperr.NotFound()

func badQuery() error { return apperr.BadRequest("request.malformed") }
