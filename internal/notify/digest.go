package notify

import (
	"context"
	"database/sql"
	"time"

	"github.com/marekbeckmann/torq/internal/i18n"
	"github.com/marekbeckmann/torq/internal/notify/presets"
	"github.com/marekbeckmann/torq/internal/reminders/engine"
	"github.com/marekbeckmann/torq/internal/store"
)

// lastWeeklySlot is the most recent instant <= now at the configured weekday
// (1 = Monday … 7 = Sunday) and hour, in now's location.
func lastWeeklySlot(now time.Time, weekday, hour int) time.Time {
	wd := int(now.Weekday())
	if wd == 0 {
		wd = 7
	}
	slot := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location()).AddDate(0, 0, -((wd - weekday + 7) % 7))
	if slot.After(now) {
		slot = slot.AddDate(0, 0, -7)
	}
	return slot
}

// lastMonthlySlot is the most recent instant <= now on the configured day (1–28) and hour.
func lastMonthlySlot(now time.Time, day, hour int) time.Time {
	slot := time.Date(now.Year(), now.Month(), day, hour, 0, 0, 0, now.Location())
	if slot.After(now) {
		slot = slot.AddDate(0, -1, 0)
	}
	return slot
}

// EnqueueDigests enqueues weekly/monthly digests for subscribed endpoints whose
// last digest predates the most recent scheduled slot (FR-039). Runs on every
// scheduler tick; a missed slot during downtime is sent once on startup.
func (s *Service) EnqueueDigests(ctx context.Context) error {
	now := s.Clock.Now()
	enqueued := false
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		cfg, err := q.GetInstanceSettings(ctx)
		if err != nil {
			return err
		}
		for _, kind := range []struct {
			event string
			slot  time.Time
		}{
			{EventDigestWeekly, lastWeeklySlot(now, int(cfg.WeeklyDigestWeekday), int(cfg.WeeklyDigestHour))},
			{EventDigestMonthly, lastMonthlySlot(now, int(cfg.MonthlyDigestDay), int(cfg.MonthlyDigestHour))},
		} {
			hooks, err := q.ListSubscribedWebhooks(ctx, kind.event)
			if err != nil {
				return err
			}
			slotDate := kind.slot.Format("2006-01-02")
			for _, wh := range hooks {
				last := wh.LastWeeklyDigestOn
				if kind.event == EventDigestMonthly {
					last = wh.LastMonthlyDigestOn
				}
				if last.Valid && last.String >= slotDate {
					continue
				}
				msg, err := s.digestMessage(ctx, q, kind.event, ownerLocale(ctx, q, wh))
				if err != nil {
					return err
				}
				if _, err := s.insert(ctx, q, wh, kind.event, sql.NullInt64{}, msg); err != nil {
					return err
				}
				mark := sql.NullString{String: slotDate, Valid: true}
				if kind.event == EventDigestWeekly {
					err = q.SetWeeklyDigestSent(ctx, store.SetWeeklyDigestSentParams{LastWeeklyDigestOn: mark, ID: wh.ID})
				} else {
					err = q.SetMonthlyDigestSent(ctx, store.SetMonthlyDigestSentParams{LastMonthlyDigestOn: mark, ID: wh.ID})
				}
				if err != nil {
					return err
				}
				enqueued = true
			}
		}
		return nil
	})
	if err == nil && enqueued {
		s.Wake()
	}
	return err
}

// digestMessage summarizes every due-soon and overdue reminder in the garage.
// An empty digest still sends "Nothing due".
func (s *Service) digestMessage(ctx context.Context, q *store.Queries, event string, loc i18n.Locale) (presets.Message, error) {
	rows, err := q.ListDueReminders(ctx)
	if err != nil {
		return presets.Message{}, err
	}
	period := "weekly"
	titleKey := "notify.digest_weekly.title"
	if event == EventDigestMonthly {
		period, titleKey = "monthly", "notify.digest_monthly.title"
	}
	d := &presets.Digest{Period: period, Overdue: []presets.DigestItem{}, DueSoon: []presets.DigestItem{}}
	for _, row := range rows {
		r := reminderRow{
			Reminder: store.Reminder{
				ID: row.ID, VehicleID: row.VehicleID, ServiceCategoryID: row.ServiceCategoryID, Title: row.Title,
				NextDueKm: row.NextDueKm, NextDueDate: row.NextDueDate, State: row.State, StateTrigger: row.StateTrigger,
			},
			Make: row.Make, Model: row.Model, LicensePlate: row.LicensePlate, CurrentOdometerKm: row.CurrentOdometerKm,
		}
		rem, veh, line := s.describe(ctx, q, r, loc)
		item := presets.DigestItem{Reminder: rem, Vehicle: veh, Line: line}
		if row.State == engine.Overdue {
			d.Overdue = append(d.Overdue, item)
		} else {
			d.DueSoon = append(d.DueSoon, item)
		}
	}
	body := loc.T("notify.digest.nothing", nil)
	if len(rows) > 0 {
		body = loc.T("notify.digest.summary", map[string]string{"overdue": loc.Number(int64(len(d.Overdue))), "due_soon": loc.Number(int64(len(d.DueSoon)))})
	}
	return presets.Message{
		Event: event, Severity: presets.SeverityInfo, Title: loc.T(titleKey, nil), Body: body,
		URL: s.link("/reminders"), Digest: d, SentAt: s.Clock.Now(),
		MoreText: func(n int) string {
			return loc.T("notify.digest.more", map[string]string{"count": loc.Number(int64(n))})
		},
	}, nil
}
