package notify

import (
	"context"
	"database/sql"
	"errors"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/notify/presets"
	"github.com/app-torq/torq/internal/store"
)

// SendTest sends a sample notification synchronously and records it in the
// delivery log (FR-040). It never retries.
func (s *Service) SendTest(ctx context.Context, webhookID int64) (Result, error) {
	q := store.New(s.DB.Writer)
	wh, err := q.GetWebhook(ctx, webhookID)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, apperr.NotFound()
	}
	if err != nil {
		return Result{}, err
	}
	loc := ownerLocale(ctx, q, wh)
	msg := presets.Message{
		Event: EventTest, Severity: presets.SeverityInfo, Title: loc.T("notify.test.title", nil),
		Body: loc.T("notify.test.body", nil), URL: s.link("/"), SentAt: s.Clock.Now(),
	}
	// Use the first vehicle and reminder as realistic sample data when available.
	vehicles, err := q.ListVehicles(ctx, 0)
	if err != nil {
		return Result{}, err
	}
	if len(vehicles) > 0 {
		v := vehicles[0]
		veh := presets.Vehicle{ID: v.ID, Name: v.Make + " " + v.Model, LicensePlate: ptrStr(v.LicensePlate), CurrentOdometerKm: v.CurrentOdometerKm}
		msg.Vehicle = &veh
		msg.Fields = []presets.Field{
			{Name: loc.T("notify.field.vehicle", nil), Value: veh.Name},
			{Name: loc.T("notify.field.odometer", nil), Value: loc.Km(veh.CurrentOdometerKm)},
		}
		if rems, err := q.ListRemindersForVehicle(ctx, v.ID); err == nil && len(rems) > 0 {
			if r, err := loadReminder(ctx, q, rems[0].ID); err == nil {
				rem, _, line := s.describe(ctx, q, r, loc)
				msg.Reminder = &rem
				msg.Body += "\n" + rem.Name + ": " + line
			}
		}
	} else {
		veh := presets.Vehicle{Name: loc.T("sample.vehicle", nil), CurrentOdometerKm: 123456}
		msg.Vehicle = &veh
	}
	id, err := s.insert(ctx, q, wh, EventTest, sql.NullInt64{}, msg)
	if err != nil {
		return Result{}, err
	}
	d, err := q.GetDelivery(ctx, id)
	if err != nil {
		return Result{}, err
	}
	return s.attempt(ctx, d, false)
}
