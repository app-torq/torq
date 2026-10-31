// Package reminders manages reminders and evaluates their state whenever
// odometer readings, service history or time change (FR-027–FR-034).
package reminders

import (
	"context"
	"database/sql"
	"errors"

	"github.com/marekbeckmann/torq/internal/clock"
	"github.com/marekbeckmann/torq/internal/db"
	"github.com/marekbeckmann/torq/internal/reminders/engine"
	"github.com/marekbeckmann/torq/internal/store"
	"github.com/marekbeckmann/torq/internal/vehicles"
)

// Transition is a reminder entering due_soon or overdue that should notify.
type Transition struct {
	ReminderID int64
	VehicleID  int64
	From       string
	To         string
}

// Service owns reminders.
type Service struct {
	DB       *db.DB
	Clock    clock.Clock
	Vehicles *vehicles.Service
	// OnTransitions runs in the evaluating transaction for every batch of
	// notify-worthy transitions (the webhook outbox installs itself here).
	OnTransitions func(ctx context.Context, tx *sql.Tx, transitions []Transition) error
}

// Baseline sources reported to clients.
const (
	BaselineServiceLog = "service_log"
	BaselineMarkDone   = "mark_done"
	BaselineStart      = "start"
)

func ptr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func intervalOf(r store.Reminder) engine.Interval {
	return engine.Interval{Km: ptr(r.IntervalKm), Months: ptr(r.IntervalMonths), Days: ptr(r.IntervalDays)}
}

func leadOf(r store.Reminder) engine.Lead {
	return engine.Lead{Km: ptr(r.LeadKm), Days: ptr(r.LeadDays)}
}

// resolveBaseline returns the effective baseline (data-model §6) and its source.
func resolveBaseline(ctx context.Context, q *store.Queries, r store.Reminder) (engine.Baseline, string, error) {
	if r.ServiceCategoryID.Valid {
		row, err := q.LatestServiceForCategory(ctx, store.LatestServiceForCategoryParams{
			VehicleID: r.VehicleID, CategoryID: r.ServiceCategoryID.Int64,
		})
		switch {
		case err == nil:
			d, perr := engine.ParseDate(row.ServiceDate)
			if perr != nil {
				return engine.Baseline{}, "", perr
			}
			return engine.Baseline{Date: d, Km: row.OdometerKm}, BaselineServiceLog, nil
		case !errors.Is(err, sql.ErrNoRows):
			return engine.Baseline{}, "", err
		}
	} else if r.LastDoneDate.Valid && r.LastDoneOdometerKm.Valid {
		d, err := engine.ParseDate(r.LastDoneDate.String)
		if err != nil {
			return engine.Baseline{}, "", err
		}
		return engine.Baseline{Date: d, Km: r.LastDoneOdometerKm.Int64}, BaselineMarkDone, nil
	}
	d, err := engine.ParseDate(r.StartDate)
	if err != nil {
		return engine.Baseline{}, "", err
	}
	return engine.Baseline{Date: d, Km: r.StartOdometerKm}, BaselineStart, nil
}

// EvaluateVehicle recomputes due points and states for every reminder of the
// vehicle inside tx and reports notify-worthy transitions to OnTransitions.
//
// Notification idempotency (R10): a transition is emitted only when the new
// state is more urgent than last_notified_state. Returning to upcoming resets
// last_notified_state so a later re-entry notifies again; a downgrade from
// overdue to due_soon is recorded silently. Paused reminders and reminders of
// archived vehicles are evaluated for display but never notify.
func (s *Service) EvaluateVehicle(ctx context.Context, tx *sql.Tx, vehicleID int64) error {
	q := store.New(tx)
	v, err := vehicles.Get(ctx, q, vehicleID)
	if err != nil {
		return err
	}
	today, err := engine.ParseDate(s.Clock.Today())
	if err != nil {
		return err
	}
	rems, err := q.ListRemindersForVehicle(ctx, vehicleID)
	if err != nil {
		return err
	}
	now := clock.Timestamp(s.Clock.Now())
	var transitions []Transition
	for _, r := range rems {
		base, source, err := resolveBaseline(ctx, q, r)
		if err != nil {
			return err
		}
		due := engine.NextDue(base, intervalOf(r))
		st := engine.Evaluate(due, leadOf(r), v.CurrentOdometerKm, today)

		active := !r.PausedAt.Valid && !v.ArchivedAt.Valid
		notified := r.LastNotifiedState
		switch {
		case !active:
			notified = st.State
		case st.State == engine.Upcoming:
			notified = engine.Upcoming
		case st.State != r.LastNotifiedState:
			if engine.Rank(st.State) > engine.Rank(r.LastNotifiedState) {
				transitions = append(transitions, Transition{ReminderID: r.ID, VehicleID: vehicleID, From: r.State, To: st.State})
			}
			notified = st.State
		}

		changedAt := r.StateChangedAt
		if st.State != r.State {
			changedAt = now
		}
		lastDate, lastKm := sql.NullString{}, sql.NullInt64{}
		if source != BaselineStart {
			lastDate = sql.NullString{String: base.Date.String(), Valid: true}
			lastKm = sql.NullInt64{Int64: base.Km, Valid: true}
		}
		params := store.SetReminderEvaluationParams{
			LastDoneDate: lastDate, LastDoneOdometerKm: lastKm,
			State: st.State, StateChangedAt: changedAt, LastNotifiedState: notified, ID: r.ID,
		}
		if due.Km != nil {
			params.NextDueKm = sql.NullInt64{Int64: *due.Km, Valid: true}
		}
		if due.Date != nil {
			params.NextDueDate = sql.NullString{String: due.Date.String(), Valid: true}
		}
		if st.Trigger != "" {
			params.StateTrigger = sql.NullString{String: st.Trigger, Valid: true}
		}
		if err := q.SetReminderEvaluation(ctx, params); err != nil {
			return err
		}
	}
	if len(transitions) > 0 && s.OnTransitions != nil {
		return s.OnTransitions(ctx, tx, transitions)
	}
	return nil
}

// EvaluateAll re-evaluates every vehicle with reminders, one transaction each,
// so time-based states advance without user activity (FR-031).
func (s *Service) EvaluateAll(ctx context.Context) error {
	ids, err := store.New(s.DB.Reader).ListVehicleIDsWithReminders(ctx)
	if err != nil {
		return err
	}
	var firstErr error
	for _, id := range ids {
		err := s.DB.WithTx(ctx, func(tx *sql.Tx) error { return s.EvaluateVehicle(ctx, tx, id) })
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
