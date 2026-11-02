package httpapi

import (
	"net/http"

	"github.com/app-torq/torq/internal/reminders"
	"github.com/app-torq/torq/internal/reminders/engine"
	"github.com/app-torq/torq/internal/vehicles"
)

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	rows, err := s.vehicles.List(ctx, vehicles.ActiveOnly)
	if err != nil {
		return err
	}
	x, err := s.loadVehicleExtras(ctx)
	if err != nil {
		return err
	}
	vs := make([]vehicleJSON, 0, len(rows))
	for _, v := range rows {
		vs = append(vs, s.toVehicleJSON(v, x))
	}
	views, err := s.reminders.List(ctx, reminders.Filter{})
	if err != nil {
		return err
	}
	attention := make([]reminderJSON, 0)
	for _, v := range views {
		if v.Status.State == engine.Overdue || v.Status.State == engine.DueSoon {
			attention = append(attention, toReminderJSON(v))
		}
	}
	open, err := s.openRepairs(ctx)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"vehicles": vs, "attention": attention, "open_repairs": open})
	return nil
}
