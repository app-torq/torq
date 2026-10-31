package vehicles

import (
	"context"
	"database/sql"

	"github.com/marekbeckmann/torq/internal/apperr"
	"github.com/marekbeckmann/torq/internal/clock"
	"github.com/marekbeckmann/torq/internal/store"
)

// Odometer reading sources (data-model §2).
const (
	SourceInitial          = "initial"
	SourceQuickUpdate      = "quick_update"
	SourceServiceLog       = "service_log"
	SourceRepairCompletion = "repair_completion"
	SourceReminderDone     = "reminder_done"
)

// largeJumpKm is the absolute increase that requires confirmation.
const largeJumpKm = 100_000

// RecordReading stores a reading from an automatic source (service log,
// repair completion, reminder done) inside tx. The current odometer is raised
// when the reading is higher and never lowered (FR-012). Linking a reading to
// its owning record is done by the caller via the returned reading ID.
func (s *Service) RecordReading(ctx context.Context, tx *sql.Tx, vehicleID int64, date string, km int64, source string) (int64, error) {
	q := store.New(tx)
	v, err := Get(ctx, q, vehicleID)
	if err != nil {
		return 0, err
	}
	id, err := q.InsertOdometerReading(ctx, store.InsertOdometerReadingParams{
		VehicleID: vehicleID, ReadingDate: date, OdometerKm: km, Source: source, CreatedAt: clock.Timestamp(s.Clock.Now()),
	})
	if err != nil {
		return 0, err
	}
	if km > v.CurrentOdometerKm {
		if err := q.SetCurrentOdometer(ctx, store.SetCurrentOdometerParams{
			CurrentOdometerKm: km, UpdatedAt: clock.Timestamp(s.Clock.Now()), ID: vehicleID,
		}); err != nil {
			return 0, err
		}
	}
	return id, nil
}

// AfterHistoryChange re-evaluates the vehicle's reminders inside tx; callers
// that changed readings or service history must invoke it before commit.
func (s *Service) AfterHistoryChange(ctx context.Context, tx *sql.Tx, vehicleID int64) error {
	return s.afterChange(ctx, tx, vehicleID)
}

// QuickUpdateInput is a manual odometer update from the dashboard or palette.
type QuickUpdateInput struct {
	OdometerKm       int64
	ReadingDate      string // default today
	ConfirmDecrease  bool
	ConfirmLargeJump bool
}

// QuickUpdate sets the current odometer. Lowering it or jumping by more than
// double / 100,000 km requires explicit confirmation (FR-013 and edge cases).
func (s *Service) QuickUpdate(ctx context.Context, vehicleID int64, in QuickUpdateInput) (store.Vehicle, error) {
	today := s.Clock.Today()
	if in.ReadingDate == "" {
		in.ReadingDate = today
	}
	var v apperr.Validation
	v.Check(in.OdometerKm < 0, "odometer_km", "validation.non_negative")
	if _, ok := ParseDate(in.ReadingDate); !ok {
		v.Add("reading_date", "validation.date")
	} else {
		v.Check(in.ReadingDate > today, "reading_date", "validation.future_date")
	}
	if err := v.Err(); err != nil {
		return store.Vehicle{}, err
	}

	var out store.Vehicle
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		cur, err := Get(ctx, q, vehicleID)
		if err != nil {
			return err
		}
		params := map[string]any{"current_km": cur.CurrentOdometerKm}
		if in.OdometerKm < cur.CurrentOdometerKm && !in.ConfirmDecrease {
			return apperr.Conflict("odometer.decrease_requires_confirmation", params)
		}
		jump := in.OdometerKm - cur.CurrentOdometerKm
		if !in.ConfirmLargeJump && (jump > largeJumpKm || (cur.CurrentOdometerKm > 0 && in.OdometerKm > 2*cur.CurrentOdometerKm)) {
			return apperr.Conflict("odometer.large_jump_requires_confirmation", params)
		}
		now := clock.Timestamp(s.Clock.Now())
		if _, err := q.InsertOdometerReading(ctx, store.InsertOdometerReadingParams{
			VehicleID: vehicleID, ReadingDate: in.ReadingDate, OdometerKm: in.OdometerKm, Source: SourceQuickUpdate, CreatedAt: now,
		}); err != nil {
			return err
		}
		if err := q.SetCurrentOdometer(ctx, store.SetCurrentOdometerParams{CurrentOdometerKm: in.OdometerKm, UpdatedAt: now, ID: vehicleID}); err != nil {
			return err
		}
		if err := s.afterChange(ctx, tx, vehicleID); err != nil {
			return err
		}
		out, err = q.GetVehicle(ctx, vehicleID)
		return err
	})
	return out, err
}

// Readings lists a vehicle's odometer history, newest first.
func (s *Service) Readings(ctx context.Context, vehicleID int64, from, to string) ([]store.ListOdometerReadingsRow, error) {
	if _, err := s.Get(ctx, vehicleID); err != nil {
		return nil, err
	}
	var fromArg, toArg any
	if from != "" {
		fromArg = from
	}
	if to != "" {
		toArg = to
	}
	return store.New(s.DB.Reader).ListOdometerReadings(ctx, store.ListOdometerReadingsParams{VehicleID: vehicleID, FromDate: fromArg, ToDate: toArg})
}
