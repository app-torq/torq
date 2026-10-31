package notify

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/marekbeckmann/torq/internal/clock"
	"github.com/marekbeckmann/torq/internal/store"
)

const (
	pollInterval   = 30 * time.Second
	maxAttempts    = 4 // 1 + 3 retries
	excerptLimit   = 2048
	deliveryStatus = "pending"
)

// Result of one delivery attempt.
type Result struct {
	Status          string  `json:"status"` // succeeded | failed | pending (retry scheduled)
	StatusCode      *int    `json:"status_code"`
	Error           *string `json:"error"`
	ResponseExcerpt string  `json:"response_excerpt"`
	DurationMs      int64   `json:"duration_ms"`
}

// Run delivers pending notifications until ctx is cancelled.
func (s *Service) Run(ctx context.Context) {
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for {
		if err := s.ProcessDue(ctx); err != nil && ctx.Err() == nil {
			s.Log.ErrorContext(ctx, "webhook delivery loop", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-t.C:
		}
	}
}

// ProcessDue sends every pending delivery whose next attempt is due.
func (s *Service) ProcessDue(ctx context.Context) error {
	for {
		// Read through the writer so rows enqueued by a still-open transaction
		// become visible only after it commits.
		rows, err := store.New(s.DB.Writer).ClaimDueDeliveries(ctx, sql.NullString{String: clock.Timestamp(s.Clock.Now()), Valid: true})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, d := range rows {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if _, err := s.attempt(ctx, d, true); err != nil {
				return err
			}
		}
		if len(rows) < 20 {
			return nil
		}
	}
}

// attempt sends one delivery and records the outcome. With retry=false a
// failure is final (test notifications).
func (s *Service) attempt(ctx context.Context, d store.WebhookDelivery, retry bool) (Result, error) {
	q := store.New(s.DB.Writer)
	wh, err := q.GetWebhook(ctx, d.WebhookID)
	if err != nil {
		return Result{}, err
	}
	custom, err := q.ListWebhookHeaders(ctx, wh.ID)
	if err != nil {
		return Result{}, err
	}
	preset := map[string]string{}
	if err := json.Unmarshal([]byte(d.RequestHeadersJson), &preset); err != nil {
		return Result{}, err
	}

	res := s.send(ctx, wh.Url, preset, custom, d.RequestBody)
	n := d.AttemptCount + 1
	params := store.RecordAttemptParams{
		AttemptCount: n, LastError: "", ResponseExcerpt: res.ResponseExcerpt,
		DurationMs: sql.NullInt64{Int64: res.DurationMs, Valid: true}, ID: d.ID,
	}
	if res.StatusCode != nil {
		params.LastStatusCode = sql.NullInt64{Int64: int64(*res.StatusCode), Valid: true}
	}
	if res.Error != nil {
		params.LastError = *res.Error
	}
	now := s.Clock.Now()
	switch {
	case res.Status == "succeeded":
		params.Status = "succeeded"
		params.CompletedAt = sql.NullString{String: clock.Timestamp(now), Valid: true}
	case retry && retryable(res) && n < maxAttempts && int(n-1) < len(s.RetryDelays):
		params.Status = deliveryStatus
		params.NextAttemptAt = sql.NullString{String: clock.Timestamp(now.Add(s.RetryDelays[n-1])), Valid: true}
		res.Status = deliveryStatus
	default:
		params.Status = "failed"
		res.Status = "failed"
		params.CompletedAt = sql.NullString{String: clock.Timestamp(now), Valid: true}
	}
	if err := q.RecordAttempt(ctx, params); err != nil {
		return Result{}, err
	}
	if params.Status != deliveryStatus {
		if _, err := q.PruneDeliveries(ctx); err != nil {
			return Result{}, err
		}
	}
	if res.Status == "failed" {
		s.Log.WarnContext(ctx, "webhook delivery failed", "webhook_id", wh.ID, "event", d.Event, "attempts", n, "status_code", params.LastStatusCode.Int64, "error", params.LastError)
	}
	return res, nil
}

// retryable: network errors, timeouts, 408, 429 and 5xx. Other 4xx are
// configuration errors that won't fix themselves.
func retryable(r Result) bool {
	if r.StatusCode == nil {
		return true
	}
	c := *r.StatusCode
	return c == http.StatusRequestTimeout || c == http.StatusTooManyRequests || c >= 500
}

func (s *Service) send(ctx context.Context, url string, preset map[string]string, custom []store.WebhookHeader, body []byte) Result {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		e := "other"
		return Result{Status: "failed", Error: &e}
	}
	for k, v := range preset {
		req.Header.Set(k, v)
	}
	for _, h := range custom {
		req.Header.Set(h.Name, h.Value)
	}
	req.Header.Set("User-Agent", "torq/"+s.Version)
	resp, err := s.Client.Do(req)
	res := Result{DurationMs: time.Since(start).Milliseconds()}
	if err != nil {
		class := errorClass(err)
		res.Error = &class
		res.Status = "failed"
		return res
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, excerptLimit))
	// The 2 KiB cut can split a rune; replace invalid bytes so the log stays valid text.
	res.ResponseExcerpt = strings.ToValidUTF8(string(b), "\uFFFD")
	code := resp.StatusCode
	res.StatusCode = &code
	if code >= 200 && code < 300 {
		res.Status = "succeeded"
	} else {
		res.Status = "failed"
	}
	return res
}

// errorClass maps transport errors to a small, stable set (never the raw
// message, which can contain URLs with tokens).
func errorClass(err error) string {
	var netErr net.Error
	var dnsErr *net.DNSError
	var certErr *tls.CertificateVerificationError
	var unknownAuth x509.UnknownAuthorityError
	var hostErr x509.HostnameError
	var recordErr tls.RecordHeaderError
	switch {
	case errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()):
		return "timeout"
	case errors.As(err, &dnsErr):
		return "dns"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection_refused"
	case errors.As(err, &certErr) || errors.As(err, &unknownAuth) || errors.As(err, &hostErr) || errors.As(err, &recordErr):
		return "tls"
	default:
		return "other"
	}
}
