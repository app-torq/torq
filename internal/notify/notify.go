// Package notify delivers reminder notifications through outbound webhooks
// only (FR-035). Deliveries are written to an outbox table in the same
// transaction as the reminder transition and sent by a single worker
// (research R10).
package notify

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/db"
	"github.com/app-torq/torq/internal/i18n"
	"github.com/app-torq/torq/internal/notify/presets"
	"github.com/app-torq/torq/internal/reminders/engine"
	"github.com/app-torq/torq/internal/store"
)

// Events (data-model §7).
const (
	EventDueSoon       = "reminder_due_soon"
	EventOverdue       = "reminder_overdue"
	EventDigestWeekly  = "digest_weekly"
	EventDigestMonthly = "digest_monthly"
	EventTest          = "test"
)

// Service owns webhook endpoints and deliveries.
type Service struct {
	DB      *db.DB
	Clock   clock.Clock
	BaseURL string
	Version string
	Log     *slog.Logger
	Client  *http.Client
	wake    chan struct{}
	// RetryDelays are the waits before retries 1..3.
	RetryDelays []time.Duration
}

// New creates the service with the delivery HTTP client (10 s timeout, no redirects).
func New(d *db.DB, clk clock.Clock, baseURL, version string, log *slog.Logger) *Service {
	return &Service{
		DB: d, Clock: clk, BaseURL: baseURL, Version: version, Log: log,
		Client: &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		wake:        make(chan struct{}, 1),
		RetryDelays: []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute},
	}
}

// Wake asks the worker to look for due deliveries now (non-blocking).
func (s *Service) Wake() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// ownerLocale returns the formatting preferences of the endpoint's owner.
func ownerLocale(ctx context.Context, q *store.Queries, wh store.Webhook) i18n.Locale {
	if !wh.OwnerUserID.Valid {
		return i18n.Default
	}
	u, err := q.GetUser(ctx, wh.OwnerUserID.Int64)
	if err != nil {
		return i18n.Default
	}
	return i18n.Locale{Lang: u.Locale, DateFormat: u.DateFormat, NumberFormat: u.NumberFormat}
}

// reminderRow is the reminder + vehicle data needed to describe a reminder.
type reminderRow struct {
	store.Reminder
	Make, Model       string
	LicensePlate      sql.NullString
	CurrentOdometerKm int64
}

func ptrInt(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func ptrStr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	v := n.String
	return &v
}

func reminderName(ctx context.Context, q *store.Queries, r store.Reminder, loc i18n.Locale) string {
	if r.Title != "" || !r.ServiceCategoryID.Valid {
		return r.Title
	}
	c, err := q.GetCategory(ctx, r.ServiceCategoryID.Int64)
	if err != nil {
		return ""
	}
	if c.BuiltinKey.Valid {
		return loc.T("categories."+c.BuiltinKey.String, nil)
	}
	return c.CustomName.String
}

// describe builds the reminder/vehicle payload parts and a localized one-line summary.
func (s *Service) describe(ctx context.Context, q *store.Queries, r reminderRow, loc i18n.Locale) (presets.Reminder, presets.Vehicle, string) {
	rem := presets.Reminder{
		ID: r.ID, Name: reminderName(ctx, q, r.Reminder, loc), State: r.State, Trigger: ptrStr(r.StateTrigger),
		NextDueKm: ptrInt(r.NextDueKm), NextDueDate: ptrStr(r.NextDueDate),
	}
	if rem.NextDueKm != nil {
		v := *rem.NextDueKm - r.CurrentOdometerKm
		rem.RemainingKm = &v
	}
	if rem.NextDueDate != nil {
		if due, err := engine.ParseDate(*rem.NextDueDate); err == nil {
			if today, err := engine.ParseDate(s.Clock.Today()); err == nil {
				v := int64(today.DaysUntil(due))
				rem.RemainingDays = &v
			}
		}
	}
	veh := presets.Vehicle{ID: r.VehicleID, Name: r.Make + " " + r.Model, LicensePlate: ptrStr(r.LicensePlate), CurrentOdometerKm: r.CurrentOdometerKm}

	var parts []string
	if rem.RemainingKm != nil {
		if *rem.RemainingKm >= 0 {
			parts = append(parts, loc.T("notify.remaining_km", map[string]string{"value": loc.Km(*rem.RemainingKm)}))
		} else {
			parts = append(parts, loc.T("notify.over_km", map[string]string{"value": loc.Km(-*rem.RemainingKm)}))
		}
	}
	if rem.RemainingDays != nil {
		switch d := *rem.RemainingDays; {
		case d == 0:
			parts = append(parts, loc.T("notify.due_today", nil))
		case d > 0:
			parts = append(parts, loc.T("notify.remaining_days", map[string]string{"value": loc.Number(d)}))
		default:
			parts = append(parts, loc.T("notify.over_days", map[string]string{"value": loc.Number(-d)}))
		}
	}
	var due []string
	if rem.NextDueKm != nil {
		due = append(due, loc.T("notify.due_at_km", map[string]string{"value": loc.Km(*rem.NextDueKm)}))
	}
	if rem.NextDueDate != nil {
		due = append(due, loc.T("notify.due_on", map[string]string{"value": loc.Date(*rem.NextDueDate)}))
	}
	line := join(parts, " · ")
	if len(due) > 0 {
		line += " (" + join(due, ", ") + ")"
	}
	return rem, veh, line
}

func join(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

func (s *Service) link(path string) string {
	if s.BaseURL == "" {
		return ""
	}
	return s.BaseURL + path
}

// reminderMessage builds the localized message for a reminder transition.
func (s *Service) reminderMessage(ctx context.Context, q *store.Queries, event string, r reminderRow, loc i18n.Locale) presets.Message {
	rem, veh, line := s.describe(ctx, q, r, loc)
	severity, titleKey := presets.SeverityWarning, "notify.due_soon.title"
	if event == EventOverdue {
		severity, titleKey = presets.SeverityCritical, "notify.overdue.title"
	}
	return presets.Message{
		Event: event, Severity: severity,
		Title: loc.T(titleKey, map[string]string{"reminder": rem.Name, "vehicle": veh.Name}),
		Body:  line, URL: s.link("/vehicles/" + strconv.FormatInt(veh.ID, 10) + "/reminders"),
		Reminder: &rem, Vehicle: &veh,
		Fields: []presets.Field{
			{Name: loc.T("notify.field.vehicle", nil), Value: veh.Name},
			{Name: loc.T("notify.field.odometer", nil), Value: loc.Km(veh.CurrentOdometerKm)},
		},
		SentAt: s.Clock.Now(),
	}
}

func loadReminder(ctx context.Context, q *store.Queries, id int64) (reminderRow, error) {
	row, err := q.GetReminderWithVehicle(ctx, id)
	if err != nil {
		return reminderRow{}, err
	}
	return reminderRow{
		Reminder: store.Reminder{
			ID: row.ID, VehicleID: row.VehicleID, ServiceCategoryID: row.ServiceCategoryID, Title: row.Title,
			NextDueKm: row.NextDueKm, NextDueDate: row.NextDueDate, State: row.State, StateTrigger: row.StateTrigger,
		},
		Make: row.Make, Model: row.Model, LicensePlate: row.LicensePlate, CurrentOdometerKm: row.CurrentOdometerKm,
	}, nil
}
