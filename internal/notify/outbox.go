package notify

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/notify/presets"
	"github.com/app-torq/torq/internal/reminders"
	"github.com/app-torq/torq/internal/reminders/engine"
	"github.com/app-torq/torq/internal/store"
)

// EnqueueTransitions is installed as reminders.Service.OnTransitions. Inside
// the evaluating transaction it inserts one pending delivery per enabled,
// subscribed endpoint for every transition (exactly-once enqueueing).
func (s *Service) EnqueueTransitions(ctx context.Context, tx *sql.Tx, transitions []reminders.Transition) error {
	q := store.New(tx)
	enqueued := false
	for _, t := range transitions {
		event := EventDueSoon
		if t.To == engine.Overdue {
			event = EventOverdue
		}
		hooks, err := q.ListSubscribedWebhooks(ctx, event)
		if err != nil {
			return err
		}
		if len(hooks) == 0 {
			continue
		}
		r, err := loadReminder(ctx, q, t.ReminderID)
		if err != nil {
			return err
		}
		for _, wh := range hooks {
			msg := s.reminderMessage(ctx, q, event, r, ownerLocale(ctx, q, wh))
			if _, err := s.insert(ctx, q, wh, event, sql.NullInt64{Int64: r.ID, Valid: true}, msg); err != nil {
				return err
			}
			enqueued = true
		}
	}
	if enqueued {
		// The worker reads through the single writer connection, so it only
		// sees these rows after this transaction commits.
		s.Wake()
	}
	return nil
}

// insert renders msg for the endpoint's preset and stores a pending delivery.
func (s *Service) insert(ctx context.Context, q *store.Queries, wh store.Webhook, event string, reminderID sql.NullInt64, msg presets.Message) (int64, error) {
	rendered, err := presets.Render(wh.Preset, msg)
	if err != nil {
		return 0, err
	}
	headers, err := json.Marshal(rendered.Headers)
	if err != nil {
		return 0, err
	}
	now := clock.Timestamp(s.Clock.Now())
	return q.InsertDelivery(ctx, store.InsertDeliveryParams{
		WebhookID: wh.ID, Event: event, ReminderID: reminderID, RequestBody: rendered.Body,
		RequestHeadersJson: string(headers), Status: "pending", AttemptCount: 0,
		NextAttemptAt: sql.NullString{String: now, Valid: true}, CreatedAt: now,
	})
}
