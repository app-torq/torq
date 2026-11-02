package notify

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/notify/presets"
	"github.com/app-torq/torq/internal/store"
)

// tokenRe matches an RFC 7230 header field name.
var tokenRe = regexp.MustCompile("^[!#$%&'*+\\-.^_`|~0-9A-Za-z]+$")

var forbiddenHeaders = map[string]bool{"host": true, "content-length": true, "transfer-encoding": true, "connection": true}

// HeaderInput sets a custom header; a nil Value keeps the stored value (FR-042).
type HeaderInput struct {
	Name  string
	Value *string
}

// WebhookInput defines an endpoint.
type WebhookInput struct {
	Name           string
	URL            string
	Preset         string
	Enabled        bool
	DueSoon        bool
	Overdue        bool
	WeeklyDigest   bool
	MonthlyDigest  bool
	Headers        []HeaderInput
	headersPresent bool
}

// WithHeaders marks that the header list should replace the stored one.
func (in WebhookInput) WithHeaders(h []HeaderInput) WebhookInput {
	in.Headers, in.headersPresent = h, true
	return in
}

// Webhook is an endpoint with header names only (values are write-only).
type Webhook struct {
	store.Webhook
	HeaderNames []string
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func validateWebhook(in *WebhookInput) error {
	var v apperr.Validation
	in.Name = strings.TrimSpace(in.Name)
	n := utf8.RuneCountInString(in.Name)
	v.Check(n < 1 || n > 64, "name", "validation.length")
	in.URL = strings.TrimSpace(in.URL)
	u, err := url.Parse(in.URL)
	v.Check(err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || len(in.URL) > 2048, "url", "validation.url")
	v.Check(!presets.Valid(in.Preset), "preset", "validation.invalid_choice")
	seen := map[string]bool{}
	for i := range in.Headers {
		h := &in.Headers[i]
		h.Name = strings.TrimSpace(h.Name)
		lower := strings.ToLower(h.Name)
		if !tokenRe.MatchString(h.Name) || forbiddenHeaders[lower] || seen[lower] {
			v.Add("headers", "validation.header_name")
		}
		seen[lower] = true
		if h.Value != nil && (strings.ContainsAny(*h.Value, "\r\n") || len(*h.Value) > 4096) {
			v.Add("headers", "validation.header_value")
		}
	}
	return v.Err()
}

func (s *Service) writeHeaders(ctx context.Context, q *store.Queries, webhookID int64, headers []HeaderInput) error {
	existing, err := q.ListWebhookHeaders(ctx, webhookID)
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, h := range headers {
		keep[strings.ToLower(h.Name)] = true
		if h.Value == nil {
			found := false
			for _, e := range existing {
				if strings.EqualFold(e.Name, h.Name) {
					found = true
				}
			}
			if !found {
				var v apperr.Validation
				v.Add("headers", "validation.required")
				return v.Err()
			}
			continue
		}
		if err := q.UpsertWebhookHeader(ctx, store.UpsertWebhookHeaderParams{WebhookID: webhookID, Name: h.Name, Value: *h.Value}); err != nil {
			return err
		}
	}
	for _, e := range existing {
		if !keep[strings.ToLower(e.Name)] {
			if err := q.DeleteWebhookHeader(ctx, e.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// Create adds an endpoint owned by ownerID (whose locale renders its messages).
func (s *Service) Create(ctx context.Context, in WebhookInput, ownerID int64) (Webhook, error) {
	if err := validateWebhook(&in); err != nil {
		return Webhook{}, err
	}
	var out Webhook
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		now := s.Clock.Now()
		// Start digests from today so a new endpoint does not get last period's digest.
		today := sql.NullString{String: s.Clock.Today(), Valid: true}
		wh, err := q.CreateWebhook(ctx, store.CreateWebhookParams{
			Name: in.Name, Url: in.URL, Preset: in.Preset, Enabled: b2i(in.Enabled),
			OnDueSoon: b2i(in.DueSoon), OnOverdue: b2i(in.Overdue), OnWeeklyDigest: b2i(in.WeeklyDigest), OnMonthlyDigest: b2i(in.MonthlyDigest),
			OwnerUserID: sql.NullInt64{Int64: ownerID, Valid: ownerID > 0}, LastWeeklyDigestOn: today, LastMonthlyDigestOn: today,
			CreatedAt: clock.Timestamp(now), UpdatedAt: clock.Timestamp(now),
		})
		if err != nil {
			return err
		}
		if err := s.writeHeaders(ctx, q, wh.ID, in.Headers); err != nil {
			return err
		}
		out, err = load(ctx, q, wh.ID)
		return err
	})
	return out, err
}

// Update replaces an endpoint's settings; headers are replaced only when provided.
func (s *Service) Update(ctx context.Context, id int64, in WebhookInput) (Webhook, error) {
	if err := validateWebhook(&in); err != nil {
		return Webhook{}, err
	}
	var out Webhook
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if _, err := q.UpdateWebhook(ctx, store.UpdateWebhookParams{
			Name: in.Name, Url: in.URL, Preset: in.Preset, Enabled: b2i(in.Enabled),
			OnDueSoon: b2i(in.DueSoon), OnOverdue: b2i(in.Overdue), OnWeeklyDigest: b2i(in.WeeklyDigest), OnMonthlyDigest: b2i(in.MonthlyDigest),
			UpdatedAt: clock.Timestamp(s.Clock.Now()), ID: id,
		}); errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		} else if err != nil {
			return err
		}
		if in.headersPresent {
			if err := s.writeHeaders(ctx, q, id, in.Headers); err != nil {
				return err
			}
		}
		var err error
		out, err = load(ctx, q, id)
		return err
	})
	return out, err
}

func load(ctx context.Context, q *store.Queries, id int64) (Webhook, error) {
	wh, err := q.GetWebhook(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Webhook{}, apperr.NotFound()
	}
	if err != nil {
		return Webhook{}, err
	}
	headers, err := q.ListWebhookHeaders(ctx, id)
	if err != nil {
		return Webhook{}, err
	}
	out := Webhook{Webhook: wh, HeaderNames: []string{}}
	for _, h := range headers {
		out.HeaderNames = append(out.HeaderNames, h.Name)
	}
	return out, nil
}

// Get returns one endpoint.
func (s *Service) Get(ctx context.Context, id int64) (Webhook, error) {
	return load(ctx, store.New(s.DB.Reader), id)
}

// List returns all endpoints.
func (s *Service) List(ctx context.Context) ([]Webhook, error) {
	q := store.New(s.DB.Reader)
	rows, err := q.ListWebhooks(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Webhook, 0, len(rows))
	for _, r := range rows {
		wh, err := load(ctx, q, r.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, wh)
	}
	return out, nil
}

// Delete removes an endpoint with its headers and delivery log.
func (s *Service) Delete(ctx context.Context, id int64) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	return store.New(s.DB.Writer).DeleteWebhook(ctx, id)
}

// Deliveries returns a page of the endpoint's delivery log.
func (s *Service) Deliveries(ctx context.Context, id, limit, offset int64) ([]store.WebhookDelivery, int64, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return nil, 0, err
	}
	q := store.New(s.DB.Reader)
	rows, err := q.ListDeliveries(ctx, store.ListDeliveriesParams{WebhookID: id, Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := q.CountDeliveries(ctx, id)
	return rows, total, err
}

// Prune trims finished deliveries to the newest 100 per endpoint.
func (s *Service) Prune(ctx context.Context) error {
	_, err := store.New(s.DB.Writer).PruneDeliveries(ctx)
	return err
}
