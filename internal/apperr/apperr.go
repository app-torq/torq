// Package apperr defines the error type shared by domain packages and the HTTP
// layer. Errors carry machine-readable codes only; the frontend translates them
// (Constitution V).
package apperr

import (
	"errors"
	"net/http"
)

// Error is a client-facing error with an HTTP status and a dotted code.
type Error struct {
	Status int
	Code   string
	Fields map[string]string
	Params map[string]any
}

func (e *Error) Error() string { return e.Code }

// As extracts an *Error from err.
func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}

func BadRequest(code string) *Error { return &Error{Status: http.StatusBadRequest, Code: code} }
func Unauthenticated() *Error {
	return &Error{Status: http.StatusUnauthorized, Code: "auth.unauthenticated"}
}
func Forbidden(code string) *Error { return &Error{Status: http.StatusForbidden, Code: code} }
func NotFound() *Error             { return &Error{Status: http.StatusNotFound, Code: "not_found"} }
func Unsupported(code string) *Error {
	return &Error{Status: http.StatusUnsupportedMediaType, Code: code}
}

// Conflict reports a business-rule conflict (409).
func Conflict(code string, params map[string]any) *Error {
	return &Error{Status: http.StatusConflict, Code: code, Params: params}
}

// Unprocessable reports a single domain validation failure (422) without field details.
func Unprocessable(code string, params map[string]any) *Error {
	return &Error{Status: http.StatusUnprocessableEntity, Code: code, Params: params}
}

// TooLarge reports an oversized upload (413).
func TooLarge(maxMB int) *Error {
	return &Error{Status: http.StatusRequestEntityTooLarge, Code: "attachment.too_large", Params: map[string]any{"max_mb": maxMB}}
}

// Throttled reports a rate-limited request (429).
func Throttled(retryAfterSeconds int) *Error {
	return &Error{Status: http.StatusTooManyRequests, Code: "auth.throttled", Params: map[string]any{"retry_after_s": retryAfterSeconds}}
}

// Validation collects field errors and reports them as one 422.
type Validation struct{ fields map[string]string }

// Add records code for field unless that field already has an error.
func (v *Validation) Add(field, code string) {
	if v.fields == nil {
		v.fields = map[string]string{}
	}
	if _, exists := v.fields[field]; !exists {
		v.fields[field] = code
	}
}

// Check records code for field when bad is true.
func (v *Validation) Check(bad bool, field, code string) {
	if bad {
		v.Add(field, code)
	}
}

// Err returns nil when no field failed, otherwise a 422 validation.failed error.
func (v *Validation) Err() error {
	if len(v.fields) == 0 {
		return nil
	}
	return &Error{Status: http.StatusUnprocessableEntity, Code: "validation.failed", Fields: v.fields}
}
