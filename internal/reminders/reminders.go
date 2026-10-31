package reminders

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/marekbeckmann/torq/internal/apperr"
	"github.com/marekbeckmann/torq/internal/clock"
	"github.com/marekbeckmann/torq/internal/reminders/engine"
	"github.com/marekbeckmann/torq/internal/store"
	"github.com/marekbeckmann/torq/internal/vehicles"
)

const (
	defaultLeadKm   = 1000
	defaultLeadDays = 30
)

// Input defines a reminder. On create, nil leads take defaults and nil
// intervals on a linked reminder take the category's defaults.
type Input struct {
	VehicleID         int64
	ServiceCategoryID *int64
	Title             string
	IntervalKm        *int64
	IntervalMonths    *int64
	IntervalDays      *int64
	LeadKm            *int64
	LeadDays          *int64
	StartDate         string
	StartOdometerKm   *int64
}

func nullInt(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func (s *Service) validate(in *Input, v *apperr.Validation) {
	in.Title = strings.TrimSpace(in.Title)
	n := utf8.RuneCountInString(in.Title)
	if in.ServiceCategoryID == nil {
		v.Check(n < 1, "title", "reminder.title_required")
	}
	v.Check(n > 120, "title", "validation.length")
	if in.IntervalKm == nil && in.IntervalMonths == nil && in.IntervalDays == nil {
		v.Add("interval_km", "reminder.interval_required")
	}
	if in.IntervalMonths != nil && in.IntervalDays != nil {
		v.Add("interval_days", "reminder.months_and_days_exclusive")
	}
	v.Check(in.IntervalKm != nil && *in.IntervalKm <= 0, "interval_km", "validation.positive")
	v.Check(in.IntervalMonths != nil && (*in.IntervalMonths < 1 || *in.IntervalMonths > 240), "interval_months", "validation.range")
	v.Check(in.IntervalDays != nil && (*in.IntervalDays < 1 || *in.IntervalDays > 3650), "interval_days", "validation.range")
	v.Check(in.LeadKm != nil && *in.LeadKm < 0, "lead_km", "validation.non_negative")
	v.Check(in.LeadDays != nil && *in.LeadDays < 0, "lead_days", "validation.non_negative")
	if in.LeadKm != nil && in.IntervalKm != nil && *in.LeadKm >= *in.IntervalKm {
		v.Add("lead_km", "reminder.lead_exceeds_interval")
	}
	iv := engine.Interval{Months: in.IntervalMonths, Days: in.IntervalDays}
	if in.LeadDays != nil && iv.HasTime() && *in.LeadDays >= iv.LengthDaysLowerBound() {
		v.Add("lead_days", "reminder.lead_exceeds_interval")
	}
	if _, err := engine.ParseDate(in.StartDate); err != nil {
		v.Add("start_date", "validation.date")
	} else {
		v.Check(in.StartDate > s.Clock.Today(), "start_date", "validation.future_date")
	}
	v.Check(in.StartOdometerKm != nil && *in.StartOdometerKm < 0, "start_odometer_km", "validation.non_negative")
}

func checkCategory(ctx context.Context, q *store.Queries, id *int64, requireActive bool) error {
	if id == nil {
		return nil
	}
	c, err := q.GetCategory(ctx, *id)
	if errors.Is(err, sql.ErrNoRows) {
		var v apperr.Validation
		v.Add("service_category_id", "validation.invalid_choice")
		return v.Err()
	}
	if err != nil {
		return err
	}
	if requireActive && c.ArchivedAt.Valid {
		var v apperr.Validation
		v.Add("service_category_id", "service_log.category_archived")
		return v.Err()
	}
	return nil
}

// Create adds a reminder and evaluates it immediately.
func (s *Service) Create(ctx context.Context, in Input, createdBy int64) (store.Reminder, error) {
	var out store.Reminder
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		veh, err := vehicles.Get(ctx, q, in.VehicleID)
		if err != nil {
			if e, ok := apperr.As(err); ok && e.Code == "not_found" {
				var v apperr.Validation
				v.Add("vehicle_id", "validation.invalid_choice")
				return v.Err()
			}
			return err
		}
		if err := checkCategory(ctx, q, in.ServiceCategoryID, true); err != nil {
			return err
		}
		if in.ServiceCategoryID != nil && in.IntervalKm == nil && in.IntervalMonths == nil && in.IntervalDays == nil {
			c, err := q.GetCategory(ctx, *in.ServiceCategoryID)
			if err != nil {
				return err
			}
			in.IntervalKm, in.IntervalMonths = ptr(c.DefaultIntervalKm), ptr(c.DefaultIntervalMonths)
		}
		if in.StartDate == "" {
			in.StartDate = s.Clock.Today()
		}
		if in.StartOdometerKm == nil {
			km := veh.CurrentOdometerKm
			in.StartOdometerKm = &km
		}
		if in.LeadKm == nil && in.IntervalKm != nil {
			lead := min(int64(defaultLeadKm), *in.IntervalKm-1)
			in.LeadKm = &lead
		}
		iv := engine.Interval{Months: in.IntervalMonths, Days: in.IntervalDays}
		if in.LeadDays == nil && iv.HasTime() {
			lead := min(int64(defaultLeadDays), iv.LengthDaysLowerBound()-1)
			in.LeadDays = &lead
		}
		var v apperr.Validation
		s.validate(&in, &v)
		if err := v.Err(); err != nil {
			return err
		}
		now := clock.Timestamp(s.Clock.Now())
		r, err := q.CreateReminder(ctx, store.CreateReminderParams{
			VehicleID: in.VehicleID, ServiceCategoryID: nullInt(in.ServiceCategoryID), Title: in.Title,
			IntervalKm: nullInt(in.IntervalKm), IntervalMonths: nullInt(in.IntervalMonths), IntervalDays: nullInt(in.IntervalDays),
			LeadKm: nullInt(in.LeadKm), LeadDays: nullInt(in.LeadDays),
			StartDate: in.StartDate, StartOdometerKm: *in.StartOdometerKm, StateChangedAt: now,
			CreatedBy: sql.NullInt64{Int64: createdBy, Valid: createdBy > 0}, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		if err := s.EvaluateVehicle(ctx, tx, in.VehicleID); err != nil {
			return err
		}
		out, err = q.GetReminder(ctx, r.ID)
		return err
	})
	return out, err
}

// Get returns one reminder.
func (s *Service) Get(ctx context.Context, id int64) (store.Reminder, error) {
	r, err := store.New(s.DB.Reader).GetReminder(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return store.Reminder{}, apperr.NotFound()
	}
	return r, err
}

// Update replaces a reminder's definition (the vehicle cannot change) and re-evaluates it.
func (s *Service) Update(ctx context.Context, id int64, in Input) (store.Reminder, error) {
	var out store.Reminder
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		cur, err := q.GetReminder(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		in.VehicleID = cur.VehicleID
		if in.StartDate == "" {
			in.StartDate = cur.StartDate
		}
		if in.StartOdometerKm == nil {
			km := cur.StartOdometerKm
			in.StartOdometerKm = &km
		}
		changedCategory := in.ServiceCategoryID != nil && (!cur.ServiceCategoryID.Valid || cur.ServiceCategoryID.Int64 != *in.ServiceCategoryID)
		if err := checkCategory(ctx, q, in.ServiceCategoryID, changedCategory); err != nil {
			return err
		}
		var v apperr.Validation
		s.validate(&in, &v)
		if err := v.Err(); err != nil {
			return err
		}
		if _, err := q.UpdateReminder(ctx, store.UpdateReminderParams{
			ServiceCategoryID: nullInt(in.ServiceCategoryID), Title: in.Title,
			IntervalKm: nullInt(in.IntervalKm), IntervalMonths: nullInt(in.IntervalMonths), IntervalDays: nullInt(in.IntervalDays),
			LeadKm: nullInt(in.LeadKm), LeadDays: nullInt(in.LeadDays),
			StartDate: in.StartDate, StartOdometerKm: *in.StartOdometerKm, UpdatedAt: clock.Timestamp(s.Clock.Now()), ID: id,
		}); err != nil {
			return err
		}
		if err := s.EvaluateVehicle(ctx, tx, cur.VehicleID); err != nil {
			return err
		}
		out, err = q.GetReminder(ctx, id)
		return err
	})
	return out, err
}

// Delete removes a reminder.
func (s *Service) Delete(ctx context.Context, id int64) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	return store.New(s.DB.Writer).DeleteReminder(ctx, id)
}

// SetPaused pauses or resumes a reminder and re-evaluates it.
func (s *Service) SetPaused(ctx context.Context, id int64, paused bool) (store.Reminder, error) {
	var out store.Reminder
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		now := clock.Timestamp(s.Clock.Now())
		at := sql.NullString{}
		if paused {
			at = sql.NullString{String: now, Valid: true}
		}
		r, err := q.SetReminderPaused(ctx, store.SetReminderPausedParams{PausedAt: at, UpdatedAt: now, ID: id})
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if err := s.EvaluateVehicle(ctx, tx, r.VehicleID); err != nil {
			return err
		}
		out, err = q.GetReminder(ctx, id)
		return err
	})
	return out, err
}

// MarkDone resets a standalone reminder from the given date and odometer
// (defaults: today, current odometer) and records an odometer reading (FR-032).
func (s *Service) MarkDone(ctx context.Context, id int64, doneOn string, odometerKm *int64) (store.Reminder, error) {
	var out store.Reminder
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		r, err := q.GetReminder(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		if r.ServiceCategoryID.Valid {
			return apperr.Conflict("reminder.linked_use_service_log", nil)
		}
		veh, err := vehicles.Get(ctx, q, r.VehicleID)
		if err != nil {
			return err
		}
		if doneOn == "" {
			doneOn = s.Clock.Today()
		}
		km := veh.CurrentOdometerKm
		if odometerKm != nil {
			km = *odometerKm
		}
		var v apperr.Validation
		if _, err := engine.ParseDate(doneOn); err != nil {
			v.Add("done_on", "validation.date")
		} else {
			v.Check(doneOn > s.Clock.Today(), "done_on", "validation.future_date")
		}
		v.Check(km < 0, "odometer_km", "validation.non_negative")
		if err := v.Err(); err != nil {
			return err
		}
		if _, err := s.Vehicles.RecordReading(ctx, tx, r.VehicleID, doneOn, km, vehicles.SourceReminderDone); err != nil {
			return err
		}
		if err := q.SetReminderDone(ctx, store.SetReminderDoneParams{
			LastDoneDate: sql.NullString{String: doneOn, Valid: true}, LastDoneOdometerKm: sql.NullInt64{Int64: km, Valid: true},
			UpdatedAt: clock.Timestamp(s.Clock.Now()), ID: id,
		}); err != nil {
			return err
		}
		if err := s.Vehicles.AfterHistoryChange(ctx, tx, r.VehicleID); err != nil {
			return err
		}
		out, err = q.GetReminder(ctx, id)
		return err
	})
	return out, err
}

// Filter selects reminders for listing.
type Filter struct {
	VehicleID       int64
	State           string
	IncludePaused   bool
	IncludeArchived bool
}

// View is a reminder with values computed for the current odometer and date.
type View struct {
	store.Reminder
	Baseline       engine.Baseline
	BaselineSource string
	Status         engine.Status
}

// List returns matching reminders, most urgent first.
func (s *Service) List(ctx context.Context, f Filter) ([]View, error) {
	q := store.New(s.DB.Reader)
	rows, err := q.ListReminders(ctx, store.ListRemindersParams{
		VehicleID: f.VehicleID, State: f.State,
		IncludePaused: boolInt(f.IncludePaused), IncludeArchived: boolInt(f.IncludeArchived),
	})
	if err != nil {
		return nil, err
	}
	views, err := s.views(ctx, q, rows)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(views, func(i, j int) bool { return lessUrgent(views[j], views[i]) })
	return views, nil
}

// ViewOf computes the view for a single reminder.
func (s *Service) ViewOf(ctx context.Context, r store.Reminder) (View, error) {
	vs, err := s.views(ctx, store.New(s.DB.Reader), []store.Reminder{r})
	if err != nil {
		return View{}, err
	}
	return vs[0], nil
}

func (s *Service) views(ctx context.Context, q *store.Queries, rows []store.Reminder) ([]View, error) {
	today, err := engine.ParseDate(s.Clock.Today())
	if err != nil {
		return nil, err
	}
	kms := map[int64]int64{}
	out := make([]View, 0, len(rows))
	for _, r := range rows {
		km, ok := kms[r.VehicleID]
		if !ok {
			v, err := vehicles.Get(ctx, q, r.VehicleID)
			if err != nil {
				return nil, err
			}
			km = v.CurrentOdometerKm
			kms[r.VehicleID] = km
		}
		base, source, err := resolveBaseline(ctx, q, r)
		if err != nil {
			return nil, err
		}
		st := engine.Evaluate(engine.NextDue(base, intervalOf(r)), leadOf(r), km, today)
		// The stored state is authoritative (it drives notifications); the
		// remaining values are computed fresh for display.
		st.State = r.State
		if r.StateTrigger.Valid {
			st.Trigger = r.StateTrigger.String
		} else {
			st.Trigger = ""
		}
		out = append(out, View{Reminder: r, Baseline: base, BaselineSource: source, Status: st})
	}
	return out, nil
}

// lessUrgent orders by state rank, then by the smaller remaining distance/time.
func lessUrgent(a, b View) bool {
	ra, rb := engine.Rank(a.Status.State), engine.Rank(b.Status.State)
	if ra != rb {
		return ra < rb
	}
	return urgencyScore(a) > urgencyScore(b)
}

// urgencyScore approximates closeness to due: lower is more urgent. One day ≈ 50 km.
func urgencyScore(v View) int64 {
	score := int64(1 << 62)
	if v.Status.RemainingKm != nil {
		score = min(score, *v.Status.RemainingKm)
	}
	if v.Status.RemainingDays != nil {
		score = min(score, *v.Status.RemainingDays*50)
	}
	return score
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// CountsByVehicle returns due-soon/overdue counts for active reminders.
func (s *Service) CountsByVehicle(ctx context.Context) (map[int64][2]int64, error) {
	rows, err := store.New(s.DB.Reader).CountReminderStatesByVehicle(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[int64][2]int64, len(rows))
	for _, r := range rows {
		out[r.VehicleID] = [2]int64{r.DueSoon, r.Overdue}
	}
	return out, nil
}
