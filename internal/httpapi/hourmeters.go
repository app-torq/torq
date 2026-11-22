package httpapi

import (
	"net/http"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/store"
	"github.com/app-torq/torq/internal/vehicles"
)

type hourMeterJSON struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	CurrentHours float64 `json:"current_hours"`
}

func toHourMeterJSON(m store.HourMeter) hourMeterJSON {
	return hourMeterJSON{ID: m.ID, Name: m.Name, CurrentHours: vehicles.TenthsToHours(m.CurrentHoursTenths)}
}

func toHourMetersJSON(ms []store.HourMeter) []hourMeterJSON {
	out := make([]hourMeterJSON, 0, len(ms))
	for _, m := range ms {
		out = append(out, toHourMeterJSON(m))
	}
	return out
}

// meterReadingJSON is an hour meter reading on a service entry, repair or
// reminder completion.
type meterReadingJSON struct {
	HourMeterID int64   `json:"hour_meter_id"`
	Hours       float64 `json:"hours"`
}

// hoursField converts hours from a request to tenths, adding a range error to
// v under field when they are out of range.
func hoursField(h float64, field string, v *apperr.Validation) int64 {
	t, ok := vehicles.HoursToTenths(h)
	v.Check(!ok, field, "validation.range")
	return t
}

// readingsIn converts request readings to domain readings.
func readingsIn(in []meterReadingJSON, field string, v *apperr.Validation) []vehicles.HoursReading {
	out := make([]vehicles.HoursReading, 0, len(in))
	for _, r := range in {
		out = append(out, vehicles.HoursReading{MeterID: r.HourMeterID, HoursTenths: hoursField(r.Hours, field, v)})
	}
	return out
}

// readingsOut converts stored readings to the API shape; never nil.
func readingsOut(in []vehicles.HoursReading) []meterReadingJSON {
	out := make([]meterReadingJSON, 0, len(in))
	for _, r := range in {
		out = append(out, meterReadingJSON{HourMeterID: r.MeterID, Hours: vehicles.TenthsToHours(r.HoursTenths)})
	}
	return out
}

func (s *Server) createHourMeter(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Name        string   `json:"name"`
		Hours       *float64 `json:"hours"`
		ReadingDate string   `json:"reading_date"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	var v apperr.Validation
	v.Check(in.Hours == nil, "hours", "validation.required")
	var tenths int64
	if in.Hours != nil {
		tenths = hoursField(*in.Hours, "hours", &v)
	}
	if err := v.Err(); err != nil {
		return err
	}
	m, err := s.vehicles.CreateMeter(r.Context(), id, vehicles.MeterInput{Name: in.Name, HoursTenths: tenths, ReadingDate: in.ReadingDate})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]any{"hour_meter": toHourMeterJSON(m)})
	return nil
}

func (s *Server) renameHourMeter(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	m, err := s.vehicles.RenameMeter(r.Context(), id, in.Name)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"hour_meter": toHourMeterJSON(m)})
	return nil
}

func (s *Server) deleteHourMeter(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if err := s.vehicles.DeleteMeter(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) quickUpdateHours(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Hours            *float64 `json:"hours"`
		ReadingDate      string   `json:"reading_date"`
		ConfirmDecrease  bool     `json:"confirm_decrease"`
		ConfirmLargeJump bool     `json:"confirm_large_jump"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	var v apperr.Validation
	v.Check(in.Hours == nil, "hours", "validation.required")
	var tenths int64
	if in.Hours != nil {
		tenths = hoursField(*in.Hours, "hours", &v)
	}
	if err := v.Err(); err != nil {
		return err
	}
	veh, err := s.vehicles.QuickUpdateHours(r.Context(), id, vehicles.HoursUpdate{
		HoursTenths: tenths, ReadingDate: in.ReadingDate, ConfirmDecrease: in.ConfirmDecrease, ConfirmLargeJump: in.ConfirmLargeJump,
	})
	if err != nil {
		return err
	}
	return s.writeVehicleWithReminders(w, r, http.StatusOK, veh)
}

type hoursReadingJSON struct {
	ID          int64   `json:"id"`
	ReadingDate string  `json:"reading_date"`
	Hours       float64 `json:"hours"`
	Source      string  `json:"source"`
	CreatedAt   string  `json:"created_at"`
}

func (s *Server) listHourMeterReadings(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	q := r.URL.Query()
	rows, err := s.vehicles.HoursReadings(r.Context(), id, q.Get("from"), q.Get("to"))
	if err != nil {
		return err
	}
	items := make([]hoursReadingJSON, 0, len(rows))
	for _, row := range rows {
		items = append(items, hoursReadingJSON{
			ID: row.ID, ReadingDate: row.ReadingDate, Hours: vehicles.TenthsToHours(row.HoursTenths), Source: row.Source, CreatedAt: row.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, newList(items, int64(len(items))))
	return nil
}
