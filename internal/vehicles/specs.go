package vehicles

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/marekbeckmann/torq/internal/apperr"
	"github.com/marekbeckmann/torq/internal/clock"
	"github.com/marekbeckmann/torq/internal/store"
)

// Specs is the structured part of the quick-reference spec sheet (FR-014).
// Capacities are in mL, torque in Nm, pressure in bar.
type Specs struct {
	EngineOilViscosity          string
	EngineOilSpec               string
	EngineOilCapacityMl         *int64
	TransmissionFluidSpec       string
	TransmissionFluidCapacityMl *int64
	DifferentialFluidSpec       string
	DifferentialFluidCapacityMl *int64
	BrakeFluidSpec              string
	BrakeFluidCapacityMl        *int64
	CoolantSpec                 string
	CoolantCapacityMl           *int64
	WheelLugTorqueNm            *float64
	TirePressureFrontBar        *float64
	TirePressureRearBar         *float64
	// TirePressure*MaxBar make the pressure a range (normal to full load);
	// they need the lower value and must not be below it.
	TirePressureFrontMaxBar *float64
	TirePressureRearMaxBar  *float64
	// Meta records, per spec field, where its value came from. Entries for
	// empty fields are dropped on save.
	Meta map[string]SpecMeta
}

// SpecMeta is the provenance of one spec field. IsAuto is true while the
// value is the one the auto-fill found at SourceURL; a manual change sets it
// to false.
type SpecMeta struct {
	IsAuto     bool   `json:"is_auto"`
	SourceURL  string `json:"source_url,omitempty"`
	SourceName string `json:"source_name,omitempty"`
}

// SpecFields are the keys of the structured spec sheet, as used in the API
// and in specs_meta.
var SpecFields = []string{
	"engine_oil_viscosity", "engine_oil_spec", "engine_oil_capacity_ml",
	"transmission_fluid_spec", "transmission_fluid_capacity_ml",
	"differential_fluid_spec", "differential_fluid_capacity_ml",
	"brake_fluid_spec", "brake_fluid_capacity_ml",
	"coolant_spec", "coolant_capacity_ml",
	"wheel_lug_torque_nm", "tire_pressure_front_bar", "tire_pressure_rear_bar",
	"tire_pressure_front_max_bar", "tire_pressure_rear_max_bar",
}

// filled reports which spec fields hold a value.
func (in Specs) filled() map[string]bool {
	return map[string]bool{
		"engine_oil_viscosity": in.EngineOilViscosity != "", "engine_oil_spec": in.EngineOilSpec != "",
		"engine_oil_capacity_ml":  in.EngineOilCapacityMl != nil,
		"transmission_fluid_spec": in.TransmissionFluidSpec != "", "transmission_fluid_capacity_ml": in.TransmissionFluidCapacityMl != nil,
		"differential_fluid_spec": in.DifferentialFluidSpec != "", "differential_fluid_capacity_ml": in.DifferentialFluidCapacityMl != nil,
		"brake_fluid_spec": in.BrakeFluidSpec != "", "brake_fluid_capacity_ml": in.BrakeFluidCapacityMl != nil,
		"coolant_spec": in.CoolantSpec != "", "coolant_capacity_ml": in.CoolantCapacityMl != nil,
		"wheel_lug_torque_nm": in.WheelLugTorqueNm != nil, "tire_pressure_front_bar": in.TirePressureFrontBar != nil,
		"tire_pressure_rear_bar":      in.TirePressureRearBar != nil,
		"tire_pressure_front_max_bar": in.TirePressureFrontMaxBar != nil, "tire_pressure_rear_max_bar": in.TirePressureRearMaxBar != nil,
	}
}

// ParseSpecsMeta decodes the stored specs_meta column; a damaged value reads
// as no metadata rather than failing the spec sheet.
func ParseSpecsMeta(raw string) map[string]SpecMeta {
	out := map[string]SpecMeta{}
	if json.Unmarshal([]byte(raw), &out) != nil {
		return map[string]SpecMeta{}
	}
	return out
}

// validateMeta checks the metadata of filled fields and returns it as JSON.
func validateMeta(v *apperr.Validation, meta map[string]SpecMeta, filled map[string]bool) string {
	keep := map[string]SpecMeta{}
	for field, m := range meta {
		known := false
		for _, f := range SpecFields {
			known = known || f == field
		}
		if !known {
			v.Add("meta", "validation.invalid_choice")
			continue
		}
		if !filled[field] {
			continue
		}
		m.SourceURL, m.SourceName = strings.TrimSpace(m.SourceURL), strings.TrimSpace(m.SourceName)
		if m.SourceURL != "" {
			u, err := url.Parse(m.SourceURL)
			v.Check(err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || len(m.SourceURL) > 2048, "meta."+field, "validation.url")
		}
		v.Check(m.IsAuto && m.SourceURL == "", "meta."+field, "validation.required")
		v.Check(utf8.RuneCountInString(m.SourceName) > 100, "meta."+field, "validation.length")
		keep[field] = m
	}
	b, _ := json.Marshal(keep)
	return string(b)
}

// GetSpecs returns the spec sheet and its custom entries.
func (s *Service) GetSpecs(ctx context.Context, vehicleID int64) (store.VehicleSpec, []store.VehicleSpecEntry, error) {
	q := store.New(s.DB.Reader)
	spec, err := q.GetVehicleSpecs(ctx, vehicleID)
	if errors.Is(err, sql.ErrNoRows) {
		return store.VehicleSpec{}, nil, apperr.NotFound()
	}
	if err != nil {
		return store.VehicleSpec{}, nil, err
	}
	entries, err := q.ListSpecEntries(ctx, vehicleID)
	return spec, entries, err
}

func nullFloat(p *float64) sql.NullFloat64 {
	if p == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *p, Valid: true}
}

// PutSpecs replaces the structured spec sheet.
func (s *Service) PutSpecs(ctx context.Context, vehicleID int64, in Specs) (store.VehicleSpec, error) {
	var v apperr.Validation
	for field, ml := range map[string]*int64{
		"engine_oil_capacity_ml":         in.EngineOilCapacityMl,
		"transmission_fluid_capacity_ml": in.TransmissionFluidCapacityMl,
		"differential_fluid_capacity_ml": in.DifferentialFluidCapacityMl,
		"brake_fluid_capacity_ml":        in.BrakeFluidCapacityMl,
		"coolant_capacity_ml":            in.CoolantCapacityMl,
	} {
		v.Check(ml != nil && *ml <= 0, field, "validation.positive")
	}
	v.Check(in.WheelLugTorqueNm != nil && *in.WheelLugTorqueNm <= 0, "wheel_lug_torque_nm", "validation.positive")
	for field, bar := range map[string]*float64{
		"tire_pressure_front_bar": in.TirePressureFrontBar, "tire_pressure_rear_bar": in.TirePressureRearBar,
		"tire_pressure_front_max_bar": in.TirePressureFrontMaxBar, "tire_pressure_rear_max_bar": in.TirePressureRearMaxBar,
	} {
		v.Check(bar != nil && (*bar <= 0 || *bar > 10), field, "validation.range")
	}
	// A maximum needs a lower value and must not be below it.
	for field, r := range map[string][2]*float64{
		"tire_pressure_front_max_bar": {in.TirePressureFrontBar, in.TirePressureFrontMaxBar},
		"tire_pressure_rear_max_bar":  {in.TirePressureRearBar, in.TirePressureRearMaxBar},
	} {
		v.Check(r[1] != nil && (r[0] == nil || *r[1] < *r[0]), field, "validation.range")
	}
	texts := map[string]*string{
		"engine_oil_viscosity": &in.EngineOilViscosity, "engine_oil_spec": &in.EngineOilSpec,
		"transmission_fluid_spec": &in.TransmissionFluidSpec, "differential_fluid_spec": &in.DifferentialFluidSpec,
		"brake_fluid_spec": &in.BrakeFluidSpec, "coolant_spec": &in.CoolantSpec,
	}
	for field, p := range texts {
		*p = strings.TrimSpace(*p)
		v.Check(utf8.RuneCountInString(*p) > 120, field, "validation.length")
	}
	meta := validateMeta(&v, in.Meta, in.filled())
	if err := v.Err(); err != nil {
		return store.VehicleSpec{}, err
	}
	out, err := store.New(s.DB.Writer).UpdateVehicleSpecs(ctx, store.UpdateVehicleSpecsParams{
		EngineOilViscosity: in.EngineOilViscosity, EngineOilSpec: in.EngineOilSpec, EngineOilCapacityMl: nullInt(in.EngineOilCapacityMl),
		TransmissionFluidSpec: in.TransmissionFluidSpec, TransmissionFluidCapacityMl: nullInt(in.TransmissionFluidCapacityMl),
		DifferentialFluidSpec: in.DifferentialFluidSpec, DifferentialFluidCapacityMl: nullInt(in.DifferentialFluidCapacityMl),
		BrakeFluidSpec: in.BrakeFluidSpec, BrakeFluidCapacityMl: nullInt(in.BrakeFluidCapacityMl),
		CoolantSpec: in.CoolantSpec, CoolantCapacityMl: nullInt(in.CoolantCapacityMl),
		WheelLugTorqueNm: nullFloat(in.WheelLugTorqueNm), TirePressureFrontBar: nullFloat(in.TirePressureFrontBar),
		TirePressureRearBar: nullFloat(in.TirePressureRearBar), TirePressureFrontMaxBar: nullFloat(in.TirePressureFrontMaxBar),
		TirePressureRearMaxBar: nullFloat(in.TirePressureRearMaxBar), SpecsMeta: meta, UpdatedAt: clock.Timestamp(s.Clock.Now()), VehicleID: vehicleID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return store.VehicleSpec{}, apperr.NotFound()
	}
	return out, err
}

func validateEntry(label, value string) error {
	var v apperr.Validation
	n := utf8.RuneCountInString(label)
	v.Check(n < 1 || n > 80, "label", "validation.length")
	v.Check(utf8.RuneCountInString(value) > 2000, "value", "validation.length")
	return v.Err()
}

// CreateSpecEntry appends a custom key-value entry.
func (s *Service) CreateSpecEntry(ctx context.Context, vehicleID int64, label, value string) (store.VehicleSpecEntry, error) {
	label, value = strings.TrimSpace(label), strings.TrimSpace(value)
	if err := validateEntry(label, value); err != nil {
		return store.VehicleSpecEntry{}, err
	}
	var out store.VehicleSpecEntry
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if _, err := Get(ctx, q, vehicleID); err != nil {
			return err
		}
		maxOrder, err := q.MaxSpecEntryOrder(ctx, vehicleID)
		if err != nil {
			return err
		}
		now := clock.Timestamp(s.Clock.Now())
		out, err = q.CreateSpecEntry(ctx, store.CreateSpecEntryParams{
			VehicleID: vehicleID, Label: label, Value: value, SortOrder: maxOrder + 1, CreatedAt: now, UpdatedAt: now,
		})
		return err
	})
	return out, err
}

// GetSpecEntry returns one custom entry.
func (s *Service) GetSpecEntry(ctx context.Context, id int64) (store.VehicleSpecEntry, error) {
	e, err := store.New(s.DB.Reader).GetSpecEntry(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return store.VehicleSpecEntry{}, apperr.NotFound()
	}
	return e, err
}

// UpdateSpecEntry changes label and/or value; nil leaves a field unchanged.
func (s *Service) UpdateSpecEntry(ctx context.Context, id int64, label, value *string) (store.VehicleSpecEntry, error) {
	cur, err := s.GetSpecEntry(ctx, id)
	if err != nil {
		return store.VehicleSpecEntry{}, err
	}
	l, val := cur.Label, cur.Value
	if label != nil {
		l = strings.TrimSpace(*label)
	}
	if value != nil {
		val = strings.TrimSpace(*value)
	}
	if err := validateEntry(l, val); err != nil {
		return store.VehicleSpecEntry{}, err
	}
	return store.New(s.DB.Writer).UpdateSpecEntry(ctx, store.UpdateSpecEntryParams{Label: l, Value: val, UpdatedAt: clock.Timestamp(s.Clock.Now()), ID: id})
}

// DeleteSpecEntry removes an entry (its images are removed by cascade; files after commit).
func (s *Service) DeleteSpecEntry(ctx context.Context, id int64, collect func(ctx context.Context, tx *sql.Tx) ([]string, error)) ([]string, error) {
	var files []string
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if _, err := q.GetSpecEntry(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		} else if err != nil {
			return err
		}
		if collect != nil {
			var err error
			if files, err = collect(ctx, tx); err != nil {
				return err
			}
		}
		return q.DeleteSpecEntry(ctx, id)
	})
	return files, err
}

// ReorderSpecEntries rewrites sort_order; ids must list every entry exactly once.
func (s *Service) ReorderSpecEntries(ctx context.Context, vehicleID int64, ids []int64) ([]store.VehicleSpecEntry, error) {
	var out []store.VehicleSpecEntry
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if _, err := Get(ctx, q, vehicleID); err != nil {
			return err
		}
		entries, err := q.ListSpecEntries(ctx, vehicleID)
		if err != nil {
			return err
		}
		if !sameIDs(ids, entries) {
			return apperr.Unprocessable("spec_entries.order_mismatch", nil)
		}
		for i, id := range ids {
			if err := q.SetSpecEntryOrder(ctx, store.SetSpecEntryOrderParams{SortOrder: int64(i + 1), ID: id, VehicleID: vehicleID}); err != nil {
				return err
			}
		}
		out, err = q.ListSpecEntries(ctx, vehicleID)
		return err
	})
	return out, err
}

func sameIDs(ids []int64, entries []store.VehicleSpecEntry) bool {
	if len(ids) != len(entries) {
		return false
	}
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			return false
		}
		seen[id] = true
	}
	for _, e := range entries {
		if !seen[e.ID] {
			return false
		}
	}
	return true
}
