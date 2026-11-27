package httpapi

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/auth"
	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/store"
	"github.com/app-torq/torq/internal/vehicles"
)

type reminderCounts struct {
	DueSoon int64 `json:"due_soon"`
	Overdue int64 `json:"overdue"`
}

type vehicleJSON struct {
	ID                int64           `json:"id"`
	VehicleType       string          `json:"vehicle_type"`
	Make              string          `json:"make"`
	Model             string          `json:"model"`
	ChassisCode       string          `json:"chassis_code"`
	ModelYear         *int64          `json:"model_year"`
	EngineCode        string          `json:"engine_code"`
	VIN               *string         `json:"vin"`
	LicensePlate      *string         `json:"license_plate"`
	HSN               *string         `json:"hsn"`
	TSN               *string         `json:"tsn"`
	DisplacementCC    *int64          `json:"displacement_cc"`
	PowerKW           *int64          `json:"power_kw"`
	PowerW            *int64          `json:"power_w"`
	FuelType          *string         `json:"fuel_type"`
	CurrentOdometerKm int64           `json:"current_odometer_km"`
	HasOdometer       bool            `json:"has_odometer"`
	HourMeters        []hourMeterJSON `json:"hour_meters"`
	OdometerUnit      string          `json:"odometer_unit"`
	DisplacementUnit  *string         `json:"displacement_unit"`
	PowerUnit         *string         `json:"power_unit"`
	AvatarURL         *string         `json:"avatar_url"`
	Archived          bool            `json:"archived"`
	ReminderCounts    reminderCounts  `json:"reminder_counts"`
	// Permissions are the caller's per-vehicle permissions on it.
	Permissions []auth.Permission `json:"permissions"`
	// SharedBy names who shared the vehicle with the caller, if another user
	// did (API level 13).
	SharedBy  *sharedByJSON `json:"shared_by"`
	CreatedAt string        `json:"created_at"`
	UpdatedAt string        `json:"updated_at"`
}

type sharedByJSON struct {
	DisplayName string `json:"display_name"`
}

// vehicleExtras are derived values shown on vehicle cards.
type vehicleExtras struct {
	counts  map[int64]reminderCounts
	avatar  map[int64]string
	meters  map[int64][]store.HourMeter
	access  auth.Access
	sharers map[int64]string
}

func (s *Server) toVehicleJSON(v store.Vehicle, x vehicleExtras) vehicleJSON {
	out := vehicleJSON{
		ID: v.ID, VehicleType: v.VehicleType, Make: v.Make, Model: v.Model, ChassisCode: v.ChassisCode, ModelYear: intPtr(v.ModelYear),
		EngineCode: v.EngineCode, VIN: strPtr(v.Vin), LicensePlate: strPtr(v.LicensePlate),
		HSN: strPtr(v.Hsn), TSN: strPtr(v.Tsn), DisplacementCC: intPtr(v.DisplacementCc), PowerKW: intPtr(v.PowerKw),
		PowerW: intPtr(v.PowerW), FuelType: strPtr(v.FuelType), CurrentOdometerKm: v.CurrentOdometerKm,
		HasOdometer: v.HasOdometer == 1, HourMeters: toHourMetersJSON(x.meters[v.ID]),
		OdometerUnit: v.OdometerUnit, DisplacementUnit: strPtr(v.DisplacementUnit), PowerUnit: strPtr(v.PowerUnit), Archived: v.ArchivedAt.Valid,
		ReminderCounts: x.counts[v.ID], Permissions: x.access.VehiclePermissions(v.ID), CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
	if url, ok := x.avatar[v.ID]; ok {
		out.AvatarURL = &url
	}
	if name, ok := x.sharers[v.ID]; ok {
		out.SharedBy = &sharedByJSON{DisplayName: name}
	}
	visibilityFor(x.access).vehicle(&out)
	return out
}

// loadVehicleExtras loads reminder counts and avatar URLs for vehicle cards.
func (s *Server) loadVehicleExtras(ctx context.Context) (vehicleExtras, error) {
	x := vehicleExtras{counts: map[int64]reminderCounts{}, avatar: map[int64]string{}, sharers: map[int64]string{}}
	if u, ok := auth.UserFrom(ctx); ok {
		x.access = u.Access
		for id := range u.Access.Received {
			name, ok, err := s.roles.VehicleSharer(ctx, u.ID, id)
			if err != nil {
				return x, err
			}
			if ok {
				x.sharers[id] = name
			}
		}
	}
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
	if x.meters, err = s.vehicles.AllMeters(ctx); err != nil {
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
	VehicleType       string  `json:"vehicle_type"`
	Make              string  `json:"make"`
	Model             string  `json:"model"`
	ChassisCode       string  `json:"chassis_code"`
	ModelYear         *int64  `json:"model_year"`
	EngineCode        string  `json:"engine_code"`
	VIN               *string `json:"vin"`
	LicensePlate      *string `json:"license_plate"`
	HSN               *string `json:"hsn"`
	TSN               *string `json:"tsn"`
	DisplacementCC    *int64  `json:"displacement_cc"`
	PowerKW           *int64  `json:"power_kw"`
	PowerW            *int64  `json:"power_w"`
	FuelType          *string `json:"fuel_type"`
	CurrentOdometerKm *int64  `json:"current_odometer_km"`
	OdometerUnit      *string `json:"odometer_unit"`
	DisplacementUnit  *string `json:"displacement_unit"`
	PowerUnit         *string `json:"power_unit"`
	HasOdometer       *bool   `json:"has_odometer"`
	// HourMeters adds meters on create only.
	HourMeters []struct {
		Name        string   `json:"name"`
		Hours       *float64 `json:"hours"`
		ReadingDate string   `json:"reading_date"`
	} `json:"hour_meters"`
}

func (in vehicleInput) profile() vehicles.Profile {
	p := vehicles.Profile{
		Make: in.Make, Model: in.Model, ChassisCode: in.ChassisCode, ModelYear: in.ModelYear, EngineCode: in.EngineCode,
		HSN: in.HSN, TSN: in.TSN, DisplacementCC: in.DisplacementCC, PowerKW: in.PowerKW, PowerW: in.PowerW,
		FuelType: in.FuelType, DisplacementUnit: in.DisplacementUnit, PowerUnit: in.PowerUnit,
		NoOdometer: in.HasOdometer != nil && !*in.HasOdometer, VehicleType: in.VehicleType,
	}
	if in.OdometerUnit != nil {
		p.OdometerUnit = *in.OdometerUnit
	}
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
	rows, err := s.vehicles.List(r.Context(), filter, visibleVehicles(r))
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
	if !visibilityOf(r.Context()).vehicleIDs {
		in.VIN, in.LicensePlate = nil, nil // what the user can't see stays empty (#49)
	}
	if in.OdometerUnit == nil {
		unit := currentUser(r).Prefs.DistanceUnit
		in.OdometerUnit = &unit
	}
	noOdometer := in.HasOdometer != nil && !*in.HasOdometer
	var v apperr.Validation
	v.Check(in.CurrentOdometerKm == nil && !noOdometer, "current_odometer_km", "validation.required")
	v.Check(len(in.HourMeters) > 1, "hour_meters", "hour_meter.limit_reached")
	p := in.profile()
	for _, m := range in.HourMeters {
		hours := 0.0
		v.Check(m.Hours == nil, "hour_meters.hours", "validation.required")
		if m.Hours != nil {
			hours = *m.Hours
		}
		p.InitialMeter = &vehicles.MeterInput{Name: m.Name, HoursTenths: hoursField(hours, "hour_meters.hours", &v), ReadingDate: m.ReadingDate}
	}
	if err := v.Err(); err != nil {
		return err
	}
	if in.CurrentOdometerKm == nil {
		in.CurrentOdometerKm = new(int64)
	}
	me := currentUser(r)
	var grant func(ctx context.Context, tx *sql.Tx, vehicleID int64) error
	if _, all := me.Access.Scope(); !all {
		// Users who see only assigned vehicles can edit the ones they add.
		grant = func(ctx context.Context, tx *sql.Tx, vehicleID int64) error {
			return auth.GrantVehicle(ctx, tx, clock.Timestamp(s.Clock.Now()), me.ID, vehicleID)
		}
	}
	veh, err := s.vehicles.CreateThen(r.Context(), p, *in.CurrentOdometerKm, me.ID, grant)
	if err != nil {
		return err
	}
	if grant != nil && me.Access.Vehicles != nil {
		// The response already shows what the user may do with it.
		me.Access.Vehicles[veh.ID] = auth.LevelEdit
	}
	out, err := s.vehicleOut(r.Context(), veh)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]any{"vehicle": out})
	return nil
}

func (s *Server) getVehicle(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathVehicle(r)
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
	id, err := s.pathVehicle(r)
	if err != nil {
		return err
	}
	cur, err := s.vehicles.Get(r.Context(), id)
	if err != nil {
		return err
	}
	hasOdometer := cur.HasOdometer == 1
	// PATCH: start from the stored profile and overlay supplied fields.
	in := vehicleInput{
		VehicleType: cur.VehicleType, Make: cur.Make, Model: cur.Model, ChassisCode: cur.ChassisCode, ModelYear: intPtr(cur.ModelYear),
		EngineCode: cur.EngineCode, VIN: strPtr(cur.Vin), LicensePlate: strPtr(cur.LicensePlate),
		HSN: strPtr(cur.Hsn), TSN: strPtr(cur.Tsn), DisplacementCC: intPtr(cur.DisplacementCc),
		PowerKW: intPtr(cur.PowerKw), FuelType: strPtr(cur.FuelType), OdometerUnit: &cur.OdometerUnit, HasOdometer: &hasOdometer,
		DisplacementUnit: strPtr(cur.DisplacementUnit), PowerUnit: strPtr(cur.PowerUnit),
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	if !visibilityOf(r.Context()).vehicleIDs {
		in.VIN, in.LicensePlate = strPtr(cur.Vin), strPtr(cur.LicensePlate) // can't change what it can't see (#49)
	}
	// A client that only knows power_kw sends back the rounded value; keep the
	// precise power_w then instead of overwriting it with whole kW.
	if in.PowerW == nil && in.PowerKW != nil && cur.PowerKw.Valid && *in.PowerKW == cur.PowerKw.Int64 {
		in.PowerW = intPtr(cur.PowerW)
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
	id, err := s.pathVehicle(r)
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
		id, err := s.pathVehicle(r)
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
	id, err := s.pathVehicle(r)
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
	writeJSON(w, status, map[string]any{"vehicle": out, "reminders": visibilityOf(r.Context()).reminderList(rems)})
	return nil
}

type readingJSON struct {
	ID          int64  `json:"id"`
	ReadingDate string `json:"reading_date"`
	OdometerKm  int64  `json:"odometer_km"`
	Source      string `json:"source"`
}

func (s *Server) listOdometerReadings(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathVehicle(r)
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
