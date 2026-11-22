// Package vehicles owns vehicle profiles, the odometer rules and the
// quick-reference spec sheet.
package vehicles

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/db"
	"github.com/app-torq/torq/internal/store"
)

var (
	// vinRe accepts standard 17-character VINs as well as shorter chassis
	// numbers (JDM frame numbers such as JZA80-0012345, pre-1981 VINs).
	vinRe = regexp.MustCompile(`^[A-Z0-9-]{1,32}$`)
	hsnRe = regexp.MustCompile(`^[0-9]{4}$`)
	tsnRe = regexp.MustCompile(`^[0-9A-Z]{3}$`)
)

// FuelTypes are the stored fuel_type values; the UI translates them.
var FuelTypes = []string{"diesel", "petrol", "hybrid", "electric", "lpg", "cng", "other"}

// OdometerUnits are the units a vehicle's odometer can read in. Distances are
// always stored in km; the unit only decides how they are shown and entered.
var OdometerUnits = []string{"km", "mi"}

// DisplacementUnits and PowerUnits are the units a vehicle can show its
// displacement and power in, overriding each user's preference.
var (
	DisplacementUnits = []string{"cc", "l", "ci"}
	PowerUnits        = []string{"kw", "ps", "hp"}
)

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
	// HSN/TSN are the German type-approval keys (Zulassungsbescheinigung
	// Teil I fields 2.1 and 2.2, first three characters of 2.2).
	HSN            *string
	TSN            *string
	DisplacementCC *int64
	// PowerW is the precise power; PowerKW is derived from it when set, so
	// values entered in hp or PS survive a round trip.
	PowerKW      *int64
	PowerW       *int64
	FuelType     *string
	OdometerUnit string
	// DisplacementUnit and PowerUnit are nil when the vehicle follows the
	// user's preference.
	DisplacementUnit *string
	PowerUnit        *string
	// NoOdometer marks a vehicle tracked by hours only; it then needs an hour
	// meter, and its current_odometer_km stays 0.
	NoOdometer bool
	// InitialMeter adds an hour meter on Create; Update ignores it.
	InitialMeter *MeterInput
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func (p *Profile) normalize() {
	p.Make = strings.TrimSpace(p.Make)
	p.Model = strings.TrimSpace(p.Model)
	p.ChassisCode = strings.TrimSpace(p.ChassisCode)
	p.EngineCode = strings.TrimSpace(p.EngineCode)
	p.VIN = strings.ToUpper(strings.TrimSpace(p.VIN))
	p.LicensePlate = strings.TrimSpace(p.LicensePlate)
	p.HSN = trimOrNil(p.HSN, false)
	p.TSN = trimOrNil(p.TSN, true)
	p.FuelType = trimOrNil(p.FuelType, false)
	p.DisplacementUnit = trimOrNil(p.DisplacementUnit, false)
	p.PowerUnit = trimOrNil(p.PowerUnit, false)
	if p.OdometerUnit == "" {
		p.OdometerUnit = "km"
	}
	switch {
	case p.PowerW != nil:
		kw := max(1, (*p.PowerW+500)/1000)
		p.PowerKW = &kw
	case p.PowerKW != nil:
		w := *p.PowerKW * 1000
		p.PowerW = &w
	}
}

// trimOrNil trims s (optionally upper-casing it) and turns "" into nil.
func trimOrNil(s *string, upper bool) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if upper {
		t = strings.ToUpper(t)
	}
	if t == "" {
		return nil
	}
	return &t
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
	v.Check(p.HSN != nil && !hsnRe.MatchString(*p.HSN), "hsn", "vehicle.hsn_invalid")
	v.Check(p.TSN != nil && !tsnRe.MatchString(*p.TSN), "tsn", "vehicle.tsn_invalid")
	v.Check(p.DisplacementCC != nil && (*p.DisplacementCC < 1 || *p.DisplacementCC > 20000), "displacement_cc", "vehicle.displacement_out_of_range")
	v.Check(p.PowerKW != nil && (*p.PowerKW < 1 || *p.PowerKW > 2000), "power_kw", "vehicle.power_out_of_range")
	v.Check(p.PowerW != nil && (*p.PowerW < 1 || *p.PowerW > 2_000_000), "power_w", "vehicle.power_out_of_range")
	v.Check(!slices.Contains(OdometerUnits, p.OdometerUnit), "odometer_unit", "validation.invalid_choice")
	v.Check(p.DisplacementUnit != nil && !slices.Contains(DisplacementUnits, *p.DisplacementUnit), "displacement_unit", "validation.invalid_choice")
	v.Check(p.PowerUnit != nil && !slices.Contains(PowerUnits, *p.PowerUnit), "power_unit", "validation.invalid_choice")
	v.Check(p.FuelType != nil && !slices.Contains(FuelTypes, *p.FuelType), "fuel_type", "vehicle.fuel_type_invalid")
}

func nullStringPtr(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func nullString(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func nullInt(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

// Create adds a vehicle with an empty spec sheet and an initial odometer
// reading, or, for a vehicle tracked by hours only, its hour meter.
func (s *Service) Create(ctx context.Context, p Profile, odometerKm int64, createdBy int64) (store.Vehicle, error) {
	p.normalize()
	if p.NoOdometer {
		odometerKm = 0
	}
	var v apperr.Validation
	s.validate(p, &v)
	v.Check(odometerKm < 0, "current_odometer_km", "validation.non_negative")
	if p.InitialMeter != nil {
		s.validateMeter(p.InitialMeter, &v, "hour_meters.")
	}
	if err := v.Err(); err != nil {
		return store.Vehicle{}, err
	}
	if p.NoOdometer && p.InitialMeter == nil {
		return store.Vehicle{}, apperr.Unprocessable("vehicle.tracking_required", nil)
	}
	var out store.Vehicle
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		now := clock.Timestamp(s.Clock.Now())
		var err error
		out, err = q.CreateVehicle(ctx, store.CreateVehicleParams{
			Make: p.Make, Model: p.Model, ChassisCode: p.ChassisCode, ModelYear: nullInt(p.ModelYear),
			EngineCode: p.EngineCode, Vin: nullString(p.VIN), LicensePlate: nullString(p.LicensePlate),
			Hsn: nullStringPtr(p.HSN), Tsn: nullStringPtr(p.TSN), DisplacementCc: nullInt(p.DisplacementCC),
			PowerKw: nullInt(p.PowerKW), PowerW: nullInt(p.PowerW), FuelType: nullStringPtr(p.FuelType),
			OdometerUnit: p.OdometerUnit, DisplacementUnit: nullStringPtr(p.DisplacementUnit), PowerUnit: nullStringPtr(p.PowerUnit),
			HasOdometer: boolInt(!p.NoOdometer), CurrentOdometerKm: odometerKm, CreatedBy: sql.NullInt64{Int64: createdBy, Valid: createdBy > 0},
			CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		if err := q.CreateVehicleSpecs(ctx, store.CreateVehicleSpecsParams{VehicleID: out.ID, UpdatedAt: now}); err != nil {
			return err
		}
		if p.InitialMeter != nil {
			if _, err := s.createMeter(ctx, q, out.ID, *p.InitialMeter); err != nil {
				return err
			}
		}
		if p.NoOdometer {
			return nil
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

// Update replaces the profile fields (not the odometer). Turning the odometer
// off needs an hour meter; turning it back on keeps the last odometer value.
func (s *Service) Update(ctx context.Context, id int64, p Profile) (store.Vehicle, error) {
	p.normalize()
	var v apperr.Validation
	s.validate(p, &v)
	if err := v.Err(); err != nil {
		return store.Vehicle{}, err
	}
	if p.NoOdometer {
		meters, err := MetersFor(ctx, store.New(s.DB.Reader), []int64{id})
		if err != nil {
			return store.Vehicle{}, err
		}
		if len(meters[id]) == 0 {
			if _, err := s.Get(ctx, id); err != nil {
				return store.Vehicle{}, err
			}
			return store.Vehicle{}, apperr.Unprocessable("vehicle.tracking_required", nil)
		}
	}
	var out store.Vehicle
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		cur, err := Get(ctx, q, id)
		if err != nil {
			return err
		}
		out, err = q.UpdateVehicleProfile(ctx, store.UpdateVehicleProfileParams{
			Make: p.Make, Model: p.Model, ChassisCode: p.ChassisCode, ModelYear: nullInt(p.ModelYear),
			EngineCode: p.EngineCode, Vin: nullString(p.VIN), LicensePlate: nullString(p.LicensePlate),
			Hsn: nullStringPtr(p.HSN), Tsn: nullStringPtr(p.TSN), DisplacementCc: nullInt(p.DisplacementCC),
			PowerKw: nullInt(p.PowerKW), PowerW: nullInt(p.PowerW), FuelType: nullStringPtr(p.FuelType),
			OdometerUnit: p.OdometerUnit, DisplacementUnit: nullStringPtr(p.DisplacementUnit), PowerUnit: nullStringPtr(p.PowerUnit),
			HasOdometer: boolInt(!p.NoOdometer), UpdatedAt: clock.Timestamp(s.Clock.Now()), ID: id,
		})
		if err != nil || cur.HasOdometer == out.HasOdometer {
			return err
		}
		// Reminders ignore distance on vehicles without an odometer.
		return s.afterChange(ctx, tx, id)
	})
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
