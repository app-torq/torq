package httpapi

import (
	"context"
	"net/http"

	"github.com/marekbeckmann/torq/internal/apperr"
	"github.com/marekbeckmann/torq/internal/store"
	"github.com/marekbeckmann/torq/internal/vehicles"
)

type reminderCounts struct {
	DueSoon int64 `json:"due_soon"`
	Overdue int64 `json:"overdue"`
}

type vehicleJSON struct {
	ID                int64          `json:"id"`
	Make              string         `json:"make"`
	Model             string         `json:"model"`
	ChassisCode       string         `json:"chassis_code"`
	ModelYear         *int64         `json:"model_year"`
	EngineCode        string         `json:"engine_code"`
	VIN               *string        `json:"vin"`
	LicensePlate      *string        `json:"license_plate"`
	CurrentOdometerKm int64          `json:"current_odometer_km"`
	AvatarURL         *string        `json:"avatar_url"`
	Archived          bool           `json:"archived"`
	ReminderCounts    reminderCounts `json:"reminder_counts"`
	CreatedAt         string         `json:"created_at"`
	UpdatedAt         string         `json:"updated_at"`
}

// vehicleExtras are derived values shown on vehicle cards.
type vehicleExtras struct {
	counts map[int64]reminderCounts
	avatar map[int64]string
}

func (s *Server) toVehicleJSON(v store.Vehicle, x vehicleExtras) vehicleJSON {
	out := vehicleJSON{
		ID: v.ID, Make: v.Make, Model: v.Model, ChassisCode: v.ChassisCode, ModelYear: intPtr(v.ModelYear),
		EngineCode: v.EngineCode, VIN: strPtr(v.Vin), LicensePlate: strPtr(v.LicensePlate),
		CurrentOdometerKm: v.CurrentOdometerKm, Archived: v.ArchivedAt.Valid,
		ReminderCounts: x.counts[v.ID], CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
	if url, ok := x.avatar[v.ID]; ok {
		out.AvatarURL = &url
	}
	return out
}

// loadVehicleExtras loads reminder counts and avatar URLs for vehicle cards.
func (s *Server) loadVehicleExtras(ctx context.Context) (vehicleExtras, error) {
	x := vehicleExtras{counts: map[int64]reminderCounts{}, avatar: map[int64]string{}}
	counts, err := s.reminders.CountsByVehicle(ctx)
	if err != nil {
		return x, err
	}
	for id, c := range counts {
		x.counts[id] = reminderCounts{DueSoon: c[0], Overdue: c[1]}
	}
	if x.avatar, err = s.vehicleAvatars(ctx); err != nil {
		return x, err
	}
	return x, nil
}

func (s *Server) vehicleOut(ctx context.Context, v store.Vehicle) (vehicleJSON, error) {
	x, err := s.loadVehicleExtras(ctx)
	if err != nil {
		return vehicleJSON{}, err
	}
	return s.toVehicleJSON(v, x), nil
}

type vehicleInput struct {
	Make              string  `json:"make"`
	Model             string  `json:"model"`
	ChassisCode       string  `json:"chassis_code"`
	ModelYear         *int64  `json:"model_year"`
	EngineCode        string  `json:"engine_code"`
	VIN               *string `json:"vin"`
	LicensePlate      *string `json:"license_plate"`
	CurrentOdometerKm *int64  `json:"current_odometer_km"`
}

func (in vehicleInput) profile() vehicles.Profile {
	p := vehicles.Profile{Make: in.Make, Model: in.Model, ChassisCode: in.ChassisCode, ModelYear: in.ModelYear, EngineCode: in.EngineCode}
	if in.VIN != nil {
		p.VIN = *in.VIN
	}
	if in.LicensePlate != nil {
		p.LicensePlate = *in.LicensePlate
	}
	return p
}

func (s *Server) listVehicles(w http.ResponseWriter, r *http.Request) error {
	filter := vehicles.ActiveOnly
	switch r.URL.Query().Get("archived") {
	case "", "false":
	case "true":
		filter = vehicles.ArchivedOnly
	case "all":
		filter = vehicles.All
	default:
		return apperr.BadRequest("request.malformed")
	}
	rows, err := s.vehicles.List(r.Context(), filter)
	if err != nil {
		return err
	}
	x, err := s.loadVehicleExtras(r.Context())
	if err != nil {
		return err
	}
	items := make([]vehicleJSON, 0, len(rows))
	for _, v := range rows {
		items = append(items, s.toVehicleJSON(v, x))
	}
	writeJSON(w, http.StatusOK, newList(items, int64(len(items))))
	return nil
}

func (s *Server) createVehicle(w http.ResponseWriter, r *http.Request) error {
	var in vehicleInput
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	if in.CurrentOdometerKm == nil {
		var v apperr.Validation
		v.Add("current_odometer_km", "validation.required")
		return v.Err()
	}
	v, err := s.vehicles.Create(r.Context(), in.profile(), *in.CurrentOdometerKm, currentUser(r).ID)
	if err != nil {
		return err
	}
	out, err := s.vehicleOut(r.Context(), v)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]any{"vehicle": out})
	return nil
}

func (s *Server) getVehicle(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	v, err := s.vehicles.Get(r.Context(), id)
	if err != nil {
		return err
	}
	out, err := s.vehicleOut(r.Context(), v)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"vehicle": out})
	return nil
}

func (s *Server) updateVehicle(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	cur, err := s.vehicles.Get(r.Context(), id)
	if err != nil {
		return err
	}
	// PATCH: start from the stored profile and overlay supplied fields.
	in := vehicleInput{
		Make: cur.Make, Model: cur.Model, ChassisCode: cur.ChassisCode, ModelYear: intPtr(cur.ModelYear),
		EngineCode: cur.EngineCode, VIN: strPtr(cur.Vin), LicensePlate: strPtr(cur.LicensePlate),
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	if in.CurrentOdometerKm != nil {
		return apperr.BadRequest("vehicle.odometer_use_quick_update")
	}
	v, err := s.vehicles.Update(r.Context(), id, in.profile())
	if err != nil {
		return err
	}
	out, err := s.vehicleOut(r.Context(), v)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"vehicle": out})
	return nil
}

func (s *Server) deleteVehicle(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		ConfirmName string `json:"confirm_name"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	if err := s.vehicles.Delete(r.Context(), id, in.ConfirmName); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) setVehicleArchived(archived bool) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := pathID(r, "id")
		if err != nil {
			return err
		}
		v, err := s.vehicles.SetArchived(r.Context(), id, archived)
		if err != nil {
			return err
		}
		out, err := s.vehicleOut(r.Context(), v)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, map[string]any{"vehicle": out})
		return nil
	}
}

func (s *Server) quickUpdateOdometer(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		OdometerKm       *int64 `json:"odometer_km"`
		ReadingDate      string `json:"reading_date"`
		ConfirmDecrease  bool   `json:"confirm_decrease"`
		ConfirmLargeJump bool   `json:"confirm_large_jump"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	if in.OdometerKm == nil {
		var v apperr.Validation
		v.Add("odometer_km", "validation.required")
		return v.Err()
	}
	v, err := s.vehicles.QuickUpdate(r.Context(), id, vehicles.QuickUpdateInput{
		OdometerKm: *in.OdometerKm, ReadingDate: in.ReadingDate,
		ConfirmDecrease: in.ConfirmDecrease, ConfirmLargeJump: in.ConfirmLargeJump,
	})
	if err != nil {
		return err
	}
	return s.writeVehicleWithReminders(w, r, http.StatusOK, v)
}

// writeVehicleWithReminders renders {vehicle, reminders} after an odometer change.
func (s *Server) writeVehicleWithReminders(w http.ResponseWriter, r *http.Request, status int, v store.Vehicle) error {
	out, err := s.vehicleOut(r.Context(), v)
	if err != nil {
		return err
	}
	rems, err := s.vehicleReminderSummaries(r.Context(), v.ID)
	if err != nil {
		return err
	}
	writeJSON(w, status, map[string]any{"vehicle": out, "reminders": rems})
	return nil
}

type readingJSON struct {
	ID          int64  `json:"id"`
	ReadingDate string `json:"reading_date"`
	OdometerKm  int64  `json:"odometer_km"`
	Source      string `json:"source"`
}

func (s *Server) listOdometerReadings(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	q := r.URL.Query()
	rows, err := s.vehicles.Readings(r.Context(), id, q.Get("from"), q.Get("to"))
	if err != nil {
		return err
	}
	items := make([]readingJSON, 0, len(rows))
	for _, row := range rows {
		items = append(items, readingJSON{ID: row.ID, ReadingDate: row.ReadingDate, OdometerKm: row.OdometerKm, Source: row.Source})
	}
	writeJSON(w, http.StatusOK, newList(items, int64(len(items))))
	return nil
}
