package httpapi

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/reminders"
	"github.com/app-torq/torq/internal/reminders/engine"
	"github.com/app-torq/torq/internal/vehicles"
)

type baselineJSON struct {
	Date       string `json:"date"`
	OdometerKm int64  `json:"odometer_km"`
	// Hours is null for reminders without an hour meter.
	Hours  *float64 `json:"hours"`
	Source string   `json:"source"`
}

// hoursPtr converts stored tenths to hours for the API.
func hoursPtr(n sql.NullInt64) *float64 {
	if !n.Valid {
		return nil
	}
	h := vehicles.TenthsToHours(n.Int64)
	return &h
}

func hoursOf(tenths *int64) *float64 {
	if tenths == nil {
		return nil
	}
	h := vehicles.TenthsToHours(*tenths)
	return &h
}

type reminderJSON struct {
	ID                int64        `json:"id"`
	VehicleID         int64        `json:"vehicle_id"`
	ServiceCategoryID *int64       `json:"service_category_id"`
	Title             string       `json:"title"`
	IntervalKm        *int64       `json:"interval_km"`
	IntervalMonths    *int64       `json:"interval_months"`
	IntervalDays      *int64       `json:"interval_days"`
	LeadKm            *int64       `json:"lead_km"`
	LeadDays          *int64       `json:"lead_days"`
	StartDate         string       `json:"start_date"`
	StartOdometerKm   int64        `json:"start_odometer_km"`
	LastDoneUnknown   bool         `json:"last_done_unknown"`
	Baseline          baselineJSON `json:"baseline"`
	NextDueKm         *int64       `json:"next_due_km"`
	NextDueDate       *string      `json:"next_due_date"`
	RemainingKm       *int64       `json:"remaining_km"`
	RemainingDays     *int64       `json:"remaining_days"`
	State             string       `json:"state"`
	StateTrigger      *string      `json:"state_trigger"`
	Paused            bool         `json:"paused"`
	HourMeterID       *int64       `json:"hour_meter_id"`
	IntervalHours     *int64       `json:"interval_hours"`
	LeadHours         *int64       `json:"lead_hours"`
	StartHours        *float64     `json:"start_hours"`
	LastDoneHours     *float64     `json:"last_done_hours"`
	NextDueHours      *float64     `json:"next_due_hours"`
	RemainingHours    *float64     `json:"remaining_hours"`
}

// reminderSummaryJSON is the compact form returned after odometer/service writes.
type reminderSummaryJSON = reminderJSON

func toReminderJSON(v reminders.View) reminderJSON {
	out := reminderJSON{
		ID: v.ID, VehicleID: v.VehicleID, ServiceCategoryID: intPtr(v.ServiceCategoryID), Title: v.Title,
		IntervalKm: intPtr(v.IntervalKm), IntervalMonths: intPtr(v.IntervalMonths), IntervalDays: intPtr(v.IntervalDays),
		LeadKm: intPtr(v.LeadKm), LeadDays: intPtr(v.LeadDays), StartDate: v.StartDate, StartOdometerKm: v.StartOdometerKm,
		Baseline:  baselineJSON{Date: v.Baseline.Date.String(), OdometerKm: v.Baseline.Km, Source: v.BaselineSource},
		NextDueKm: intPtr(v.NextDueKm), NextDueDate: strPtr(v.NextDueDate),
		RemainingKm: v.Status.RemainingKm, RemainingDays: v.Status.RemainingDays,
		State: v.Status.State, Paused: v.PausedAt.Valid, LastDoneUnknown: v.LastDoneUnknown == 1,
		HourMeterID: intPtr(v.HourMeterID), IntervalHours: intPtr(v.IntervalHours), LeadHours: intPtr(v.LeadHours),
		StartHours: hoursPtr(v.StartHoursTenths), LastDoneHours: hoursPtr(v.LastDoneHoursTenths),
		NextDueHours: hoursPtr(v.NextDueHoursTenths), RemainingHours: hoursOf(v.Status.RemainingHoursTenths),
	}
	if v.HourMeterID.Valid {
		h := vehicles.TenthsToHours(v.Baseline.HoursTenths)
		out.Baseline.Hours = &h
	}
	if v.Status.Trigger != "" {
		tr := v.Status.Trigger
		out.StateTrigger = &tr
	}
	return out
}

func toReminderJSONList(views []reminders.View) []reminderJSON {
	out := make([]reminderJSON, 0, len(views))
	for _, v := range views {
		out = append(out, toReminderJSON(v))
	}
	return out
}

// vehicleReminderSummaries returns the vehicle's re-evaluated reminders for
// responses to odometer and service history writes.
func (s *Server) vehicleReminderSummaries(ctx context.Context, vehicleID int64) ([]reminderSummaryJSON, error) {
	views, err := s.reminders.List(ctx, reminders.Filter{VehicleID: vehicleID, IncludePaused: true, IncludeArchived: true})
	if err != nil {
		return nil, err
	}
	return toReminderJSONList(views), nil
}

func (s *Server) listReminders(w http.ResponseWriter, r *http.Request) error {
	vehicleID, err := queryInt(r, "vehicle_id")
	if err != nil {
		return err
	}
	q := r.URL.Query()
	state := q.Get("state")
	switch state {
	case "", engine.Upcoming, engine.DueSoon, engine.Overdue:
	default:
		return badQuery()
	}
	views, err := s.reminders.List(r.Context(), reminders.Filter{
		VehicleID: vehicleID, State: state, IncludePaused: q.Get("include_paused") == "true", IncludeArchived: vehicleID != 0,
		VehicleIDs: visibleVehicles(r),
	})
	if err != nil {
		return err
	}
	items := toReminderJSONList(views)
	writeJSON(w, http.StatusOK, newList(items, int64(len(items))))
	return nil
}

type reminderInput struct {
	VehicleID         int64  `json:"vehicle_id"`
	ServiceCategoryID *int64 `json:"service_category_id"`
	Title             string `json:"title"`
	IntervalKm        *int64 `json:"interval_km"`
	IntervalMonths    *int64 `json:"interval_months"`
	IntervalDays      *int64 `json:"interval_days"`
	LeadKm            *int64 `json:"lead_km"`
	LeadDays          *int64 `json:"lead_days"`
	StartDate         string `json:"start_date"`
	StartOdometerKm   *int64 `json:"start_odometer_km"`
	LastDoneUnknown   *bool  `json:"last_done_unknown"`
	// HourMeterID: omitted keeps the hours settings on update (clients that
	// don't know about hours must not clear them); null removes them.
	HourMeterID   optionalID `json:"hour_meter_id"`
	IntervalHours *int64     `json:"interval_hours"`
	LeadHours     *int64     `json:"lead_hours"`
	StartHours    *float64   `json:"start_hours"`
}

func (in reminderInput) domain() (reminders.Input, error) {
	out := reminders.Input{
		VehicleID: in.VehicleID, ServiceCategoryID: in.ServiceCategoryID, Title: in.Title,
		IntervalKm: in.IntervalKm, IntervalMonths: in.IntervalMonths, IntervalDays: in.IntervalDays,
		LeadKm: in.LeadKm, LeadDays: in.LeadDays, StartDate: in.StartDate, StartOdometerKm: in.StartOdometerKm,
		LastDoneUnknown: in.LastDoneUnknown, HourMeterID: in.HourMeterID.Value, IntervalHours: in.IntervalHours, LeadHours: in.LeadHours,
	}
	if in.StartHours != nil {
		var v apperr.Validation
		t := hoursField(*in.StartHours, "start_hours", &v)
		if err := v.Err(); err != nil {
			return reminders.Input{}, err
		}
		out.StartHoursTenths = &t
	}
	return out, nil
}

func (s *Server) writeReminder(w http.ResponseWriter, r *http.Request, status int, id int64) error {
	rem, err := s.reminders.Get(r.Context(), id)
	if err != nil {
		return err
	}
	view, err := s.reminders.ViewOf(r.Context(), rem)
	if err != nil {
		return err
	}
	writeJSON(w, status, map[string]any{"reminder": toReminderJSON(view)})
	return nil
}

func (s *Server) createReminder(w http.ResponseWriter, r *http.Request) error {
	var in reminderInput
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	if in.VehicleID != 0 { // 0 fails validation below
		if err := s.guard(r, in.VehicleID); err != nil {
			return err
		}
	}
	d, err := in.domain()
	if err != nil {
		return err
	}
	rem, err := s.reminders.Create(r.Context(), d, currentUser(r).ID)
	if err != nil {
		return err
	}
	return s.writeReminder(w, r, http.StatusCreated, rem.ID)
}

func (s *Server) getReminder(w http.ResponseWriter, r *http.Request) error {
	id, _, err := s.pathReminder(r)
	if err != nil {
		return err
	}
	return s.writeReminder(w, r, http.StatusOK, id)
}

func (s *Server) updateReminder(w http.ResponseWriter, r *http.Request) error {
	id, vehicleID, err := s.pathReminder(r)
	if err != nil {
		return err
	}
	var in reminderInput
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	if in.VehicleID != 0 && in.VehicleID != vehicleID {
		// Moving the reminder needs the permission on both vehicles.
		if err := s.guard(r, in.VehicleID); err != nil {
			return err
		}
	}
	if !in.HourMeterID.Set {
		cur, err := s.reminders.Get(r.Context(), id)
		if err != nil {
			return err
		}
		in.HourMeterID = optionalID{Set: true, Value: intPtr(cur.HourMeterID)}
		in.IntervalHours, in.LeadHours = intPtr(cur.IntervalHours), intPtr(cur.LeadHours)
	}
	d, err := in.domain()
	if err != nil {
		return err
	}
	if _, err := s.reminders.Update(r.Context(), id, d); err != nil {
		return err
	}
	return s.writeReminder(w, r, http.StatusOK, id)
}

func (s *Server) deleteReminder(w http.ResponseWriter, r *http.Request) error {
	id, _, err := s.pathReminder(r)
	if err != nil {
		return err
	}
	if err := s.reminders.Delete(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) setReminderPaused(paused bool) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, _, err := s.pathReminder(r)
		if err != nil {
			return err
		}
		if _, err := s.reminders.SetPaused(r.Context(), id, paused); err != nil {
			return err
		}
		return s.writeReminder(w, r, http.StatusOK, id)
	}
}

func (s *Server) markReminderDone(w http.ResponseWriter, r *http.Request) error {
	id, _, err := s.pathReminder(r)
	if err != nil {
		return err
	}
	var in struct {
		DoneOn        string             `json:"done_on"`
		OdometerKm    *int64             `json:"odometer_km"`
		MeterReadings []meterReadingJSON `json:"meter_readings"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	var verr apperr.Validation
	readings := readingsIn(in.MeterReadings, "meter_readings", &verr)
	if err := verr.Err(); err != nil {
		return err
	}
	rem, err := s.reminders.MarkDoneWithReadings(r.Context(), id, in.DoneOn, in.OdometerKm, readings)
	if err != nil {
		return err
	}
	view, err := s.reminders.ViewOf(r.Context(), rem)
	if err != nil {
		return err
	}
	v, err := s.vehicles.Get(r.Context(), rem.VehicleID)
	if err != nil {
		return err
	}
	vj, err := s.vehicleOut(r.Context(), v)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"reminder": toReminderJSON(view), "vehicle": vj})
	return nil
}

// pathReminder parses the {id} wildcard as a reminder and checks its vehicle
// for the route's permission.
func (s *Server) pathReminder(r *http.Request) (id, vehicleID int64, err error) {
	if id, err = pathID(r, "id"); err != nil {
		return 0, 0, err
	}
	rem, err := s.reminders.Get(r.Context(), id)
	if err != nil {
		return 0, 0, err
	}
	return id, rem.VehicleID, s.guard(r, rem.VehicleID)
}
