// Package vehicles owns vehicle profiles, the odometer rules and the
// quick-reference spec sheet.
package vehicles

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/marekbeckmann/torq/internal/apperr"
	"github.com/marekbeckmann/torq/internal/clock"
	"github.com/marekbeckmann/torq/internal/db"
	"github.com/marekbeckmann/torq/internal/store"
)

var vinRe = regexp.MustCompile(`^[A-HJ-NPR-Z0-9]{17}$`)

// Service manages vehicles.
type Service struct {
	DB    *db.DB
	Clock clock.Clock
	// AfterOdometerChange runs inside the writing transaction whenever a
	// vehicle's odometer, history or archive state changes. The reminders
	// package installs its evaluator here.
	AfterOdometerChange func(ctx context.Context, tx *sql.Tx, vehicleID int64) error
	// BeforeDelete collects files to remove after commit (attachments).
	BeforeDelete func(ctx context.Context, tx *sql.Tx, vehicleID int64) ([]string, error)
	// AfterCommitDelete removes files collected by BeforeDelete.
	AfterCommitDelete func(keys []string)
}

// Profile is the editable vehicle profile.
type Profile struct {
	Make         string
	Model        string
	ChassisCode  string
	ModelYear    *int64
	EngineCode   string
	VIN          string
	LicensePlate string
}

func (p *Profile) normalize() {
	p.Make = strings.TrimSpace(p.Make)
	p.Model = strings.TrimSpace(p.Model)
	p.ChassisCode = strings.TrimSpace(p.ChassisCode)
	p.EngineCode = strings.TrimSpace(p.EngineCode)
	p.VIN = strings.ToUpper(strings.TrimSpace(p.VIN))
	p.LicensePlate = strings.TrimSpace(p.LicensePlate)
}

func (s *Service) validate(p Profile, v *apperr.Validation) {
	n := utf8.RuneCountInString(p.Make)
	v.Check(n < 1 || n > 64, "make", "validation.length")
	n = utf8.RuneCountInString(p.Model)
	v.Check(n < 1 || n > 64, "model", "validation.length")
	v.Check(utf8.RuneCountInString(p.ChassisCode) > 32, "chassis_code", "validation.length")
	v.Check(utf8.RuneCountInString(p.EngineCode) > 32, "engine_code", "validation.length")
	if p.ModelYear != nil {
		maxYear := int64(s.Clock.Now().Year() + 1)
		v.Check(*p.ModelYear < 1886 || *p.ModelYear > maxYear, "model_year", "validation.range")
	}
	v.Check(p.VIN != "" && !vinRe.MatchString(p.VIN), "vin", "validation.vin")
	v.Check(utf8.RuneCountInString(p.LicensePlate) > 16, "license_plate", "validation.length")
}

func nullString(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func nullInt(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

// Create adds a vehicle with an empty spec sheet and an initial odometer reading.
func (s *Service) Create(ctx context.Context, p Profile, odometerKm int64, createdBy int64) (store.Vehicle, error) {
	p.normalize()
	var v apperr.Validation
	s.validate(p, &v)
	v.Check(odometerKm < 0, "current_odometer_km", "validation.non_negative")
	if err := v.Err(); err != nil {
		return store.Vehicle{}, err
	}
	var out store.Vehicle
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		now := clock.Timestamp(s.Clock.Now())
		var err error
		out, err = q.CreateVehicle(ctx, store.CreateVehicleParams{
			Make: p.Make, Model: p.Model, ChassisCode: p.ChassisCode, ModelYear: nullInt(p.ModelYear),
			EngineCode: p.EngineCode, Vin: nullString(p.VIN), LicensePlate: nullString(p.LicensePlate),
			CurrentOdometerKm: odometerKm, CreatedBy: sql.NullInt64{Int64: createdBy, Valid: createdBy > 0},
			CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		if err := q.CreateVehicleSpecs(ctx, store.CreateVehicleSpecsParams{VehicleID: out.ID, UpdatedAt: now}); err != nil {
			return err
		}
		_, err = q.InsertOdometerReading(ctx, store.InsertOdometerReadingParams{
			VehicleID: out.ID, ReadingDate: s.Clock.Today(), OdometerKm: odometerKm, Source: SourceInitial, CreatedAt: now,
		})
		return err
	})
	return out, err
}

// Get returns one vehicle.
func (s *Service) Get(ctx context.Context, id int64) (store.Vehicle, error) {
	return Get(ctx, store.New(s.DB.Reader), id)
}

// Get loads a vehicle through q, mapping a missing row to 404.
func Get(ctx context.Context, q *store.Queries, id int64) (store.Vehicle, error) {
	v, err := q.GetVehicle(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return store.Vehicle{}, apperr.NotFound()
	}
	return v, err
}

// ArchiveFilter selects vehicles by archived state.
type ArchiveFilter int64

const (
	ActiveOnly   ArchiveFilter = 0
	ArchivedOnly ArchiveFilter = 1
	All          ArchiveFilter = 2
)

// List returns vehicles matching filter.
func (s *Service) List(ctx context.Context, filter ArchiveFilter) ([]store.Vehicle, error) {
	return store.New(s.DB.Reader).ListVehicles(ctx, int64(filter))
}

// Update replaces the profile fields (not the odometer).
func (s *Service) Update(ctx context.Context, id int64, p Profile) (store.Vehicle, error) {
	p.normalize()
	var v apperr.Validation
	s.validate(p, &v)
	if err := v.Err(); err != nil {
		return store.Vehicle{}, err
	}
	out, err := store.New(s.DB.Writer).UpdateVehicleProfile(ctx, store.UpdateVehicleProfileParams{
		Make: p.Make, Model: p.Model, ChassisCode: p.ChassisCode, ModelYear: nullInt(p.ModelYear),
		EngineCode: p.EngineCode, Vin: nullString(p.VIN), LicensePlate: nullString(p.LicensePlate),
		UpdatedAt: clock.Timestamp(s.Clock.Now()), ID: id,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return store.Vehicle{}, apperr.NotFound()
	}
	return out, err
}

// SetArchived archives or restores a vehicle and re-evaluates its reminders
// (archived vehicles have inactive reminders).
func (s *Service) SetArchived(ctx context.Context, id int64, archived bool) (store.Vehicle, error) {
	var out store.Vehicle
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		now := clock.Timestamp(s.Clock.Now())
		at := sql.NullString{}
		if archived {
			at = sql.NullString{String: now, Valid: true}
		}
		var err error
		out, err = store.New(tx).SetVehicleArchived(ctx, store.SetVehicleArchivedParams{ArchivedAt: at, UpdatedAt: now, ID: id})
		if errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		}
		if err != nil {
			return err
		}
		return s.afterChange(ctx, tx, id)
	})
	return out, err
}

// Delete removes a vehicle and everything recorded for it. confirmName must
// equal "<make> <model>" (case-insensitive).
func (s *Service) Delete(ctx context.Context, id int64, confirmName string) error {
	var files []string
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		v, err := Get(ctx, q, id)
		if err != nil {
			return err
		}
		if !strings.EqualFold(strings.TrimSpace(confirmName), v.Make+" "+v.Model) {
			return apperr.Unprocessable("vehicle.confirm_name_mismatch", nil)
		}
		if s.BeforeDelete != nil {
			if files, err = s.BeforeDelete(ctx, tx, id); err != nil {
				return err
			}
		}
		return q.DeleteVehicle(ctx, id)
	})
	if err == nil && s.AfterCommitDelete != nil && len(files) > 0 {
		s.AfterCommitDelete(files)
	}
	return err
}

func (s *Service) afterChange(ctx context.Context, tx *sql.Tx, vehicleID int64) error {
	if s.AfterOdometerChange == nil {
		return nil
	}
	return s.AfterOdometerChange(ctx, tx, vehicleID)
}

// ParseDate validates a YYYY-MM-DD calendar date.
func ParseDate(s string) (time.Time, bool) {
	t, err := time.Parse(clock.DateLayout, s)
	return t, err == nil
}
