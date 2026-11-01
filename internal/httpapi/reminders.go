package httpapi

import (
	"context"
	"net/http"

	"github.com/marekbeckmann/torq/internal/reminders"
	"github.com/marekbeckmann/torq/internal/reminders/engine"
)

type baselineJSON struct {
	Date       string `json:"date"`
	OdometerKm int64  `json:"odometer_km"`
	Source     string `json:"source"`
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
	Baseline          baselineJSON `json:"baseline"`
	NextDueKm         *int64       `json:"next_due_km"`
	NextDueDate       *string      `json:"next_due_date"`
	RemainingKm       *int64       `json:"remaining_km"`
	RemainingDays     *int64       `json:"remaining_days"`
	State             string       `json:"state"`
	StateTrigger      *string      `json:"state_trigger"`
	Paused            bool         `json:"paused"`
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
		State: v.Status.State, Paused: v.PausedAt.Valid,
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
}

func (in reminderInput) domain() reminders.Input {
	return reminders.Input{
		VehicleID: in.VehicleID, ServiceCategoryID: in.ServiceCategoryID, Title: in.Title,
		IntervalKm: in.IntervalKm, IntervalMonths: in.IntervalMonths, IntervalDays: in.IntervalDays,
		LeadKm: in.LeadKm, LeadDays: in.LeadDays, StartDate: in.StartDate, StartOdometerKm: in.StartOdometerKm,
	}
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
	rem, err := s.reminders.Create(r.Context(), in.domain(), currentUser(r).ID)
	if err != nil {
		return err
	}
	return s.writeReminder(w, r, http.StatusCreated, rem.ID)
}

func (s *Server) getReminder(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	return s.writeReminder(w, r, http.StatusOK, id)
}

func (s *Server) updateReminder(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in reminderInput
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	if _, err := s.reminders.Update(r.Context(), id, in.domain()); err != nil {
		return err
	}
	return s.writeReminder(w, r, http.StatusOK, id)
}

func (s *Server) deleteReminder(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
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
		id, err := pathID(r, "id")
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
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		DoneOn     string `json:"done_on"`
		OdometerKm *int64 `json:"odometer_km"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	rem, err := s.reminders.MarkDone(r.Context(), id, in.DoneOn, in.OdometerKm)
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
