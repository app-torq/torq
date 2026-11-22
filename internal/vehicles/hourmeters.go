package vehicles

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/store"
)

// MaxHoursTenths is the largest hour meter value, 999,999.9 h.
const MaxHoursTenths = 9_999_999

// HoursToTenths converts hours as sent by clients (at most one decimal is
// meaningful) to stored tenths, rounding to the nearest tenth. ok is false for
// negative, non-finite or too large values.
func HoursToTenths(h float64) (int64, bool) {
	if math.IsNaN(h) || math.IsInf(h, 0) || h < 0 {
		return 0, false
	}
	t := int64(math.Round(h * 10))
	return t, t <= MaxHoursTenths
}

// TenthsToHours converts stored tenths to hours for the API.
func TenthsToHours(t int64) float64 { return float64(t) / 10 }

// largeJumpHoursTenths is the absolute increase (5,000 h) that requires
// confirmation, the hours counterpart of largeJumpKm (research H5).
const largeJumpHoursTenths = 50_000

// MeterInput creates an hour meter with its first reading.
type MeterInput struct {
	Name        string
	HoursTenths int64
	ReadingDate string // default today
}

// HoursUpdate is a manual hour meter update, like QuickUpdateInput.
type HoursUpdate struct {
	HoursTenths      int64
	ReadingDate      string // default today
	ConfirmDecrease  bool
	ConfirmLargeJump bool
}

func (s *Service) validateMeter(in *MeterInput, v *apperr.Validation, prefix string) {
	in.Name = strings.TrimSpace(in.Name)
	n := utf8.RuneCountInString(in.Name)
	v.Check(n < 1 || n > 40, prefix+"name", "validation.length")
	v.Check(in.HoursTenths < 0 || in.HoursTenths > MaxHoursTenths, prefix+"hours", "validation.range")
	s.checkReadingDate(&in.ReadingDate, v, prefix+"reading_date")
}

func (s *Service) checkReadingDate(date *string, v *apperr.Validation, field string) {
	today := s.Clock.Today()
	if *date == "" {
		*date = today
	}
	if _, ok := ParseDate(*date); !ok {
		v.Add(field, "validation.date")
	} else {
		v.Check(*date > today, field, "validation.future_date")
	}
}

// getMeter loads an hour meter through q, mapping a missing row to 404.
func getMeter(ctx context.Context, q *store.Queries, id int64) (store.HourMeter, error) {
	m, err := q.GetHourMeter(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return store.HourMeter{}, apperr.NotFound()
	}
	return m, err
}

// createMeter inserts a meter and its initial reading inside q's transaction.
// A vehicle has at most one meter for now (data-model §1).
func (s *Service) createMeter(ctx context.Context, q *store.Queries, vehicleID int64, in MeterInput) (store.HourMeter, error) {
	existing, err := q.ListHourMetersForVehicles(ctx, []int64{vehicleID})
	if err != nil {
		return store.HourMeter{}, err
	}
	if len(existing) > 0 {
		return store.HourMeter{}, apperr.Conflict("hour_meter.limit_reached", nil)
	}
	now := clock.Timestamp(s.Clock.Now())
	m, err := q.CreateHourMeter(ctx, store.CreateHourMeterParams{
		VehicleID: vehicleID, Name: in.Name, CurrentHoursTenths: in.HoursTenths, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return store.HourMeter{}, err
	}
	_, err = q.InsertHourMeterReading(ctx, store.InsertHourMeterReadingParams{
		HourMeterID: m.ID, ReadingDate: in.ReadingDate, HoursTenths: in.HoursTenths, Source: SourceInitial, CreatedAt: now,
	})
	return m, err
}

// CreateMeter adds an hour meter to an existing vehicle.
func (s *Service) CreateMeter(ctx context.Context, vehicleID int64, in MeterInput) (store.HourMeter, error) {
	var v apperr.Validation
	s.validateMeter(&in, &v, "")
	if err := v.Err(); err != nil {
		return store.HourMeter{}, err
	}
	var out store.HourMeter
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if _, err := Get(ctx, q, vehicleID); err != nil {
			return err
		}
		var err error
		out, err = s.createMeter(ctx, q, vehicleID, in)
		return err
	})
	return out, err
}

// RenameMeter changes a meter's name.
func (s *Service) RenameMeter(ctx context.Context, id int64, name string) (store.HourMeter, error) {
	name = strings.TrimSpace(name)
	var v apperr.Validation
	n := utf8.RuneCountInString(name)
	v.Check(n < 1 || n > 40, "name", "validation.length")
	if err := v.Err(); err != nil {
		return store.HourMeter{}, err
	}
	m, err := store.New(s.DB.Writer).RenameHourMeter(ctx, store.RenameHourMeterParams{Name: name, UpdatedAt: clock.Timestamp(s.Clock.Now()), ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return store.HourMeter{}, apperr.NotFound()
	}
	return m, err
}

// DeleteMeter removes a meter that no service entry, repair or hours-only
// reminder depends on (research H12). Reminders that also use distance or time
// drop their hours dimension.
func (s *Service) DeleteMeter(ctx context.Context, id int64) error {
	return s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		m, err := getMeter(ctx, q, id)
		if err != nil {
			return err
		}
		v, err := Get(ctx, q, m.VehicleID)
		if err != nil {
			return err
		}
		if v.HasOdometer == 0 {
			return apperr.Conflict("hour_meter.required", nil)
		}
		used, err := q.CountHourMeterEntryReadings(ctx, id)
		if err != nil {
			return err
		}
		onlyHours, err := q.CountRemindersOnlyByHours(ctx, sql.NullInt64{Int64: id, Valid: true})
		if err != nil {
			return err
		}
		if used.ServiceLogs > 0 || used.Repairs > 0 || onlyHours > 0 {
			return apperr.Conflict("hour_meter.in_use", map[string]any{
				"service_logs": used.ServiceLogs, "repairs": used.Repairs, "reminders": onlyHours,
			})
		}
		if err := q.ClearRemindersHourMeter(ctx, store.ClearRemindersHourMeterParams{
			UpdatedAt: clock.Timestamp(s.Clock.Now()), HourMeterID: sql.NullInt64{Int64: id, Valid: true},
		}); err != nil {
			return err
		}
		if err := q.DeleteHourMeter(ctx, id); err != nil {
			return err
		}
		return s.afterChange(ctx, tx, m.VehicleID)
	})
}

// QuickUpdateHours sets a meter's current hours. Lowering it or jumping by
// more than double / 5,000 h requires explicit confirmation, as for the odometer.
func (s *Service) QuickUpdateHours(ctx context.Context, meterID int64, in HoursUpdate) (store.Vehicle, error) {
	var v apperr.Validation
	v.Check(in.HoursTenths < 0 || in.HoursTenths > MaxHoursTenths, "hours", "validation.range")
	s.checkReadingDate(&in.ReadingDate, &v, "reading_date")
	if err := v.Err(); err != nil {
		return store.Vehicle{}, err
	}
	var out store.Vehicle
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		m, err := getMeter(ctx, q, meterID)
		if err != nil {
			return err
		}
		cur := m.CurrentHoursTenths
		params := map[string]any{"current_hours": TenthsToHours(cur)}
		if in.HoursTenths < cur && !in.ConfirmDecrease {
			return apperr.Conflict("hour_meter.decrease_requires_confirmation", params)
		}
		jump := in.HoursTenths - cur
		if !in.ConfirmLargeJump && (jump > largeJumpHoursTenths || (cur > 0 && in.HoursTenths > 2*cur)) {
			return apperr.Conflict("hour_meter.large_jump_requires_confirmation", params)
		}
		now := clock.Timestamp(s.Clock.Now())
		if _, err := q.InsertHourMeterReading(ctx, store.InsertHourMeterReadingParams{
			HourMeterID: meterID, ReadingDate: in.ReadingDate, HoursTenths: in.HoursTenths, Source: SourceQuickUpdate, CreatedAt: now,
		}); err != nil {
			return err
		}
		if err := q.SetHourMeterCurrent(ctx, store.SetHourMeterCurrentParams{CurrentHoursTenths: in.HoursTenths, UpdatedAt: now, ID: meterID}); err != nil {
			return err
		}
		if err := s.afterChange(ctx, tx, m.VehicleID); err != nil {
			return err
		}
		out, err = q.GetVehicle(ctx, m.VehicleID)
		return err
	})
	return out, err
}

// HoursReading is an hour meter reading recorded by a service entry, repair
// completion or reminder completion.
type HoursReading struct {
	MeterID      int64
	HoursTenths  int64
	ServiceLogID int64 // 0 when not from a service entry
	RepairLogID  int64 // 0 when not from a repair
}

// ValidateReadings checks that readings belong to the vehicle's meters and are
// in range. field names the request field for errors. required is true for
// vehicles without an odometer, whose entries need hours instead of distance.
func ValidateReadings(ctx context.Context, q *store.Queries, vehicleID int64, readings []HoursReading, required bool, field string, v *apperr.Validation) error {
	meters, err := q.ListHourMetersForVehicles(ctx, []int64{vehicleID})
	if err != nil {
		return err
	}
	seen := map[int64]bool{}
	for _, r := range readings {
		ok := false
		for _, m := range meters {
			ok = ok || m.ID == r.MeterID
		}
		v.Check(!ok || seen[r.MeterID], field, "validation.invalid_choice")
		v.Check(r.HoursTenths < 0 || r.HoursTenths > MaxHoursTenths, field, "validation.range")
		seen[r.MeterID] = true
	}
	v.Check(required && len(readings) == 0, field, "validation.required")
	return nil
}

// RecordHoursReading stores a reading from an automatic source inside tx and
// raises the meter's current value when the reading is higher; it is never
// lowered (same rule as RecordReading).
func (s *Service) RecordHoursReading(ctx context.Context, tx *sql.Tx, date, source string, r HoursReading) error {
	q := store.New(tx)
	m, err := getMeter(ctx, q, r.MeterID)
	if err != nil {
		return err
	}
	now := clock.Timestamp(s.Clock.Now())
	if _, err := q.InsertHourMeterReading(ctx, store.InsertHourMeterReadingParams{
		HourMeterID: r.MeterID, ReadingDate: date, HoursTenths: r.HoursTenths, Source: source,
		ServiceLogID: sql.NullInt64{Int64: r.ServiceLogID, Valid: r.ServiceLogID > 0},
		RepairLogID:  sql.NullInt64{Int64: r.RepairLogID, Valid: r.RepairLogID > 0}, CreatedAt: now,
	}); err != nil {
		return err
	}
	if r.HoursTenths > m.CurrentHoursTenths {
		return q.SetHourMeterCurrent(ctx, store.SetHourMeterCurrentParams{CurrentHoursTenths: r.HoursTenths, UpdatedAt: now, ID: r.MeterID})
	}
	return nil
}

// HoursReadings lists a meter's history, newest first.
func (s *Service) HoursReadings(ctx context.Context, meterID int64, from, to string) ([]store.ListHourMeterReadingsRow, error) {
	q := store.New(s.DB.Reader)
	if _, err := getMeter(ctx, q, meterID); err != nil {
		return nil, err
	}
	var fromArg, toArg any
	if from != "" {
		fromArg = from
	}
	if to != "" {
		toArg = to
	}
	return q.ListHourMeterReadings(ctx, store.ListHourMeterReadingsParams{HourMeterID: meterID, FromDate: fromArg, ToDate: toArg})
}

// MetersFor returns the hour meters of the given vehicles, keyed by vehicle.
func MetersFor(ctx context.Context, q *store.Queries, vehicleIDs []int64) (map[int64][]store.HourMeter, error) {
	out := map[int64][]store.HourMeter{}
	if len(vehicleIDs) == 0 {
		return out, nil
	}
	rows, err := q.ListHourMetersForVehicles(ctx, vehicleIDs)
	if err != nil {
		return nil, err
	}
	for _, m := range rows {
		out[m.VehicleID] = append(out[m.VehicleID], m)
	}
	return out, nil
}

// Meters returns the hour meters of the given vehicles, keyed by vehicle.
func (s *Service) Meters(ctx context.Context, vehicleIDs []int64) (map[int64][]store.HourMeter, error) {
	return MetersFor(ctx, store.New(s.DB.Reader), vehicleIDs)
}

// AllMeters returns every hour meter, keyed by vehicle.
func (s *Service) AllMeters(ctx context.Context) (map[int64][]store.HourMeter, error) {
	rows, err := store.New(s.DB.Reader).ListAllHourMeters(ctx)
	if err != nil {
		return nil, err
	}
	out := map[int64][]store.HourMeter{}
	for _, m := range rows {
		out[m.VehicleID] = append(out[m.VehicleID], m)
	}
	return out, nil
}
