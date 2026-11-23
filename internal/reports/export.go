package reports

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/app-torq/torq/internal/i18n"
	"github.com/app-torq/torq/internal/repairs"
	"github.com/app-torq/torq/internal/servicelog"
	"github.com/app-torq/torq/internal/store"
	"github.com/app-torq/torq/internal/vehicles"
)

func sqlStr(s string) sql.NullString { return sql.NullString{String: s, Valid: true} }

// ExportOptions control what a history export contains (FR-047).
type ExportOptions struct {
	IncludeCosts bool
	IncludeNotes bool
	Locale       i18n.Locale // localizes built-in category names
}

// History is the torq.vehicle-history/v1 document (research R19).
type History struct {
	Schema   string           `json:"schema"`
	Instance HistoryInstance  `json:"instance"`
	Vehicle  HistoryVehicle   `json:"vehicle"`
	Specs    map[string]any   `json:"specs"`
	Entries  []HistoryEntry   `json:"spec_entries"`
	Services []HistoryService `json:"service_logs"`
	Repairs  []HistoryRepair  `json:"repairs"`
}

// HistoryInstance names the torq instance the history comes from (#32).
type HistoryInstance struct {
	Name string `json:"name"`
}

type HistoryVehicle struct {
	ID                int64   `json:"id"`
	VehicleType       string  `json:"vehicle_type"`
	Make              string  `json:"make"`
	Model             string  `json:"model"`
	ChassisCode       string  `json:"chassis_code"`
	ModelYear         *int64  `json:"model_year"`
	EngineCode        string  `json:"engine_code"`
	VIN               *string `json:"vin"`
	LicensePlate      *string `json:"license_plate"`
	CurrentOdometerKm int64   `json:"current_odometer_km"`
	HSN               *string `json:"hsn"`
	TSN               *string `json:"tsn"`
	DisplacementCC    *int64  `json:"displacement_cc"`
	PowerKW           *int64  `json:"power_kw"`
	PowerW            *int64  `json:"power_w"`
	FuelType          *string `json:"fuel_type"`
	// OdometerUnit is how the vehicle's odometer reads; distances stay in km.
	OdometerUnit string `json:"odometer_unit"`
	// DisplacementUnit and PowerUnit are the vehicle's display units; null
	// means the user's preference.
	DisplacementUnit *string `json:"displacement_unit"`
	PowerUnit        *string `json:"power_unit"`
	// HasOdometer is false for vehicles tracked by hours only; their
	// distances are 0 and mean nothing.
	HasOdometer bool               `json:"has_odometer"`
	HourMeters  []HistoryHourMeter `json:"hour_meters"`
}

// HistoryHourMeter is a vehicle's hour meter with its current hours.
type HistoryHourMeter struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	CurrentHours float64 `json:"current_hours"`
}

// HistoryReading is the hours of an hour meter recorded on an entry.
type HistoryReading struct {
	HourMeterID int64   `json:"hour_meter_id"`
	Hours       float64 `json:"hours"`
}

func readingsOf(in []vehicles.HoursReading) []HistoryReading {
	out := make([]HistoryReading, 0, len(in))
	for _, r := range in {
		out = append(out, HistoryReading{HourMeterID: r.MeterID, Hours: vehicles.TenthsToHours(r.HoursTenths)})
	}
	return out
}

type HistoryEntry struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type HistoryAttachment struct {
	OriginalName string `json:"original_name"`
	MimeType     string `json:"mime_type"`
	SizeBytes    int64  `json:"size_bytes"`
	Caption      string `json:"caption"`
}

type HistoryServiceItem struct {
	Category              string  `json:"category"`
	Brand                 string  `json:"brand"`
	Specification         string  `json:"specification"`
	QuantityMl            *int64  `json:"quantity_ml"`
	QuantityUnit          *string `json:"quantity_unit"`
	OEMPartNumber         string  `json:"oem_part_number"`
	AftermarketPartNumber string  `json:"aftermarket_part_number"`
	CostCents             *int64  `json:"cost_cents,omitempty"`
	CostType              string  `json:"cost_type"`
}

type HistoryService struct {
	ID               int64                `json:"id"`
	Date             string               `json:"service_date"`
	OdometerKm       int64                `json:"odometer_km"`
	OdometerRecorded bool                 `json:"odometer_recorded"`
	MeterReadings    []HistoryReading     `json:"meter_readings"`
	Notes            string               `json:"notes,omitempty"`
	ExtraCostCents   *int64               `json:"extra_cost_cents,omitempty"`
	ExtraCostLabel   string               `json:"extra_cost_label,omitempty"`
	TotalCents       *int64               `json:"total_cents,omitempty"`
	Items            []HistoryServiceItem `json:"items"`
	Attachments      []HistoryAttachment  `json:"attachments"`
}

type HistoryTorque struct {
	Label   string  `json:"label"`
	ValueNm float64 `json:"value_nm"`
	Note    string  `json:"note"`
}

type HistoryPart struct {
	Name                  string `json:"name"`
	OEMPartNumber         string `json:"oem_part_number"`
	AftermarketBrand      string `json:"aftermarket_brand"`
	AftermarketPartNumber string `json:"aftermarket_part_number"`
	Quantity              int64  `json:"quantity"`
	UnitCostCents         *int64 `json:"unit_cost_cents,omitempty"`
	CostType              string `json:"cost_type"`
	Status                string `json:"status"`
}

type HistoryRepair struct {
	ID               int64               `json:"id"`
	Title            string              `json:"title"`
	StartedOn        *string             `json:"started_on"`
	CompletedOn      string              `json:"completed_on"`
	OdometerKm       int64               `json:"completion_odometer_km"`
	OdometerRecorded bool                `json:"completion_odometer_recorded"`
	MeterReadings    []HistoryReading    `json:"completion_meter_readings"`
	Symptom          string              `json:"symptom"`
	RootCause        string              `json:"root_cause"`
	Procedure        string              `json:"procedure_notes,omitempty"`
	SpecialTools     string              `json:"special_tools"`
	FaultCodes       []string            `json:"fault_codes"`
	TorqueSpecs      []HistoryTorque     `json:"torque_specs"`
	Parts            []HistoryPart       `json:"parts"`
	PartsTotal       *int64              `json:"parts_total_cents,omitempty"`
	Attachments      []HistoryAttachment `json:"attachments"`
}

func ptrI(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func ptrS(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	v := n.String
	return &v
}

func ptrF(n sql.NullFloat64) any {
	if !n.Valid {
		return nil
	}
	return n.Float64
}

func attachmentsOf(rows []store.Attachment) []HistoryAttachment {
	out := make([]HistoryAttachment, 0, len(rows))
	for _, a := range rows {
		out = append(out, HistoryAttachment{OriginalName: a.OriginalName, MimeType: a.MimeType, SizeBytes: a.SizeBytes, Caption: a.Caption})
	}
	return out
}

// History builds the complete history document for one vehicle.
func (s *Service) History(ctx context.Context, vehicleID int64, opt ExportOptions) (History, error) {
	v, err := s.Vehicles.Get(ctx, vehicleID)
	if err != nil {
		return History{}, err
	}
	q := store.New(s.DB.Reader)
	spec, entries, err := s.Vehicles.GetSpecs(ctx, vehicleID)
	if err != nil {
		return History{}, err
	}
	settings, err := q.GetInstanceSettings(ctx)
	if err != nil {
		return History{}, err
	}
	h := History{
		Schema:   "torq.vehicle-history/v1",
		Instance: HistoryInstance{Name: settings.BrandName},
		Vehicle: HistoryVehicle{ID: v.ID, VehicleType: v.VehicleType, Make: v.Make, Model: v.Model, ChassisCode: v.ChassisCode, ModelYear: ptrI(v.ModelYear),
			EngineCode: v.EngineCode, VIN: ptrS(v.Vin), LicensePlate: ptrS(v.LicensePlate), CurrentOdometerKm: v.CurrentOdometerKm,
			HSN: ptrS(v.Hsn), TSN: ptrS(v.Tsn), DisplacementCC: ptrI(v.DisplacementCc), PowerKW: ptrI(v.PowerKw), PowerW: ptrI(v.PowerW), FuelType: ptrS(v.FuelType),
			OdometerUnit: v.OdometerUnit, DisplacementUnit: ptrS(v.DisplacementUnit), PowerUnit: ptrS(v.PowerUnit),
			HasOdometer: v.HasOdometer == 1, HourMeters: []HistoryHourMeter{}},
		Specs: map[string]any{
			"engine_oil_viscosity": spec.EngineOilViscosity, "engine_oil_spec": spec.EngineOilSpec, "engine_oil_capacity_ml": ptrI(spec.EngineOilCapacityMl),
			"transmission_fluid_spec": spec.TransmissionFluidSpec, "transmission_fluid_capacity_ml": ptrI(spec.TransmissionFluidCapacityMl),
			"differential_fluid_spec": spec.DifferentialFluidSpec, "differential_fluid_capacity_ml": ptrI(spec.DifferentialFluidCapacityMl),
			"brake_fluid_spec": spec.BrakeFluidSpec, "brake_fluid_capacity_ml": ptrI(spec.BrakeFluidCapacityMl),
			"coolant_spec": spec.CoolantSpec, "coolant_capacity_ml": ptrI(spec.CoolantCapacityMl),
			"wheel_lug_torque_nm": ptrF(spec.WheelLugTorqueNm), "tire_pressure_front_bar": ptrF(spec.TirePressureFrontBar),
			"tire_pressure_rear_bar": ptrF(spec.TirePressureRearBar), "tire_pressure_front_max_bar": ptrF(spec.TirePressureFrontMaxBar),
			"tire_pressure_rear_max_bar": ptrF(spec.TirePressureRearMaxBar),
			// Motorcycle fields (#28), stored for every vehicle type.
			"final_drive": ptrS(spec.FinalDrive), "sprocket_front_teeth": ptrI(spec.SprocketFrontTeeth),
			"sprocket_rear_teeth": ptrI(spec.SprocketRearTeeth), "chain_size": spec.ChainSize, "chain_links": ptrI(spec.ChainLinks),
			"fork_oil_spec": spec.ForkOilSpec, "fork_oil_capacity_ml": ptrI(spec.ForkOilCapacityMl),
			"valve_clearance_intake_min_mm": ptrF(spec.ValveClearanceIntakeMinMm), "valve_clearance_intake_max_mm": ptrF(spec.ValveClearanceIntakeMaxMm),
			"valve_clearance_exhaust_min_mm": ptrF(spec.ValveClearanceExhaustMinMm), "valve_clearance_exhaust_max_mm": ptrF(spec.ValveClearanceExhaustMaxMm),
		},
		Entries: []HistoryEntry{}, Services: []HistoryService{}, Repairs: []HistoryRepair{},
	}
	for _, e := range entries {
		h.Entries = append(h.Entries, HistoryEntry{Label: e.Label, Value: e.Value})
	}
	meters, err := vehicles.MetersFor(ctx, q, []int64{vehicleID})
	if err != nil {
		return History{}, err
	}
	for _, m := range meters[vehicleID] {
		h.Vehicle.HourMeters = append(h.Vehicle.HourMeters, HistoryHourMeter{ID: m.ID, Name: m.Name, CurrentHours: vehicles.TenthsToHours(m.CurrentHoursTenths)})
	}

	cats, err := s.categoryNames(ctx, q, opt.Locale)
	if err != nil {
		return History{}, err
	}
	logs, _, err := s.ServiceLog.List(ctx, servicelog.Filter{VehicleID: vehicleID, Limit: 1 << 30})
	if err != nil {
		return History{}, err
	}
	for _, l := range logs {
		atts, err := q.ListAttachmentsForServiceLog(ctx, sql.NullInt64{Int64: l.ID, Valid: true})
		if err != nil {
			return History{}, err
		}
		hs := HistoryService{ID: l.ID, Date: l.ServiceDate, OdometerKm: l.OdometerKm, OdometerRecorded: l.OdometerRecorded == 1,
			MeterReadings: readingsOf(l.MeterReadings), Items: []HistoryServiceItem{}, Attachments: attachmentsOf(atts)}
		if opt.IncludeNotes {
			hs.Notes = l.Notes
		}
		if opt.IncludeCosts {
			extra, total := l.ExtraCostCents, l.TotalCents()
			hs.ExtraCostCents, hs.TotalCents, hs.ExtraCostLabel = &extra, &total, l.ExtraCostLabel
		}
		for _, it := range l.Items {
			item := HistoryServiceItem{Category: cats[it.CategoryID], Brand: it.Brand, Specification: it.Specification,
				QuantityMl: ptrI(it.QuantityMl), QuantityUnit: ptrS(it.QuantityUnit), OEMPartNumber: it.OemPartNumber,
				AftermarketPartNumber: it.AftermarketPartNumber, CostType: it.CostType}
			if opt.IncludeCosts {
				item.CostCents = ptrI(it.CostCents)
			}
			hs.Items = append(hs.Items, item)
		}
		h.Services = append(h.Services, hs)
	}

	reps, _, err := s.Repairs.List(ctx, repairs.Filter{VehicleID: vehicleID, Statuses: []string{repairs.StatusCompleted}, Limit: 1 << 30})
	if err != nil {
		return History{}, err
	}
	for _, summary := range reps {
		r, err := s.Repairs.Get(ctx, summary.ID)
		if err != nil {
			return History{}, err
		}
		atts, err := q.ListAttachmentsForRepair(ctx, sql.NullInt64{Int64: r.ID, Valid: true})
		if err != nil {
			return History{}, err
		}
		hr := HistoryRepair{ID: r.ID, Title: r.Title, StartedOn: ptrS(r.StartedOn), CompletedOn: r.CompletedOn.String,
			OdometerKm: r.CompletionOdometerKm.Int64, OdometerRecorded: r.CompletionOdometerRecorded == 1,
			MeterReadings: readingsOf(r.CompletionMeterReadings), Symptom: r.Symptom, RootCause: r.RootCause, SpecialTools: r.SpecialTools,
			FaultCodes: r.FaultCodes, TorqueSpecs: []HistoryTorque{}, Parts: []HistoryPart{}, Attachments: attachmentsOf(atts)}
		if hr.FaultCodes == nil {
			hr.FaultCodes = []string{}
		}
		if opt.IncludeNotes {
			hr.Procedure = r.ProcedureNotes
		}
		if opt.IncludeCosts {
			total := r.Totals.TotalCents
			hr.PartsTotal = &total
		}
		for _, ts := range r.TorqueSpecs {
			hr.TorqueSpecs = append(hr.TorqueSpecs, HistoryTorque{Label: ts.Label, ValueNm: ts.ValueNm, Note: ts.Note})
		}
		for _, p := range r.Parts {
			hp := HistoryPart{Name: p.Name, OEMPartNumber: p.OemPartNumber, AftermarketBrand: p.AftermarketBrand,
				AftermarketPartNumber: p.AftermarketPartNumber, Quantity: p.Quantity, CostType: p.CostType, Status: p.Status}
			if opt.IncludeCosts {
				hp.UnitCostCents = ptrI(p.UnitCostCents)
			}
			hr.Parts = append(hr.Parts, hp)
		}
		h.Repairs = append(h.Repairs, hr)
	}
	return h, nil
}

func (s *Service) categoryNames(ctx context.Context, q *store.Queries, loc i18n.Locale) (map[int64]string, error) {
	rows, err := q.ListCategories(ctx, store.ListCategoriesParams{IncludeArchived: 1})
	if err != nil {
		return nil, err
	}
	out := make(map[int64]string, len(rows))
	for _, c := range rows {
		if c.BuiltinKey.Valid {
			out[c.ID] = loc.T("categories."+c.BuiltinKey.String, nil)
		} else {
			out[c.ID] = c.CustomName.String
		}
	}
	return out, nil
}

// CSVHeader is the fixed column order of the CSV export. The vehicle columns
// were appended later (specs/002, FR-027) and are filled only on the leading
// "vehicle" record; the hour meter columns came next (#24), then the vehicle
// type (#28, vehicle record only). Appending keeps existing columns in their
// positions.
var CSVHeader = []string{"record_type", "date", "odometer_km", "record_title", "category_or_part", "brand", "specification",
	"quantity", "unit", "oem_part_number", "aftermarket_part_number", "cost", "cost_type", "status", "notes",
	"hsn", "tsn", "displacement_cc", "power_kw", "fuel_type", "hour_meter", "hours", "vehicle_type"}

// vehicleCols is the number of vehicle columns before the hour meter columns.
const vehicleCols = 5

// hoursCols renders the hour meter columns: the meter's name and hours of
// the first reading, empty without one.
func hoursCols(meters map[int64]string, readings []HistoryReading) []string {
	if len(readings) == 0 {
		return []string{"", ""}
	}
	return []string{meters[readings[0].HourMeterID], strconv.FormatFloat(readings[0].Hours, 'f', -1, 64)}
}

// kmCol is an entry's odometer cell, empty when no distance was recorded.
func kmCol(km int64, recorded bool) string {
	if !recorded {
		return ""
	}
	return strconv.FormatInt(km, 10)
}

func strOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func intOr(p *int64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatInt(*p, 10)
}

// money renders cents as a plain decimal with a dot (machine-readable).
func money(cents *int64) string {
	if cents == nil {
		return ""
	}
	c := *cents
	sign := ""
	if c < 0 {
		sign, c = "-", -c
	}
	return fmt.Sprintf("%s%d.%02d", sign, c/100, c%100)
}

// qty returns the quantity in metric units: mL when it was entered in mL,
// otherwise L (including amounts entered in US units).
func qty(ml *int64, unit *string) (string, string) {
	if ml == nil || unit == nil {
		return "", ""
	}
	if *unit == "mL" {
		return strconv.FormatInt(*ml, 10), "mL"
	}
	return strconv.FormatFloat(float64(*ml)/1000, 'f', -1, 64), "L"
}

// neutralize prefixes text cells a spreadsheet would run as a formula
// ("=", "+", "-", "@", tab, CR) with a quote. Numbers stay as they are, so
// negative amounts remain numeric.
func neutralize(rec []string) []string {
	for i, v := range rec {
		if v == "" || !strings.ContainsRune("=+-@\t\r", rune(v[0])) {
			continue
		}
		if _, err := strconv.ParseFloat(v, 64); err == nil {
			continue
		}
		rec[i] = "'" + v
	}
	return rec
}

// WriteCSV writes the history as one flat RFC 4180 file, UTF-8 with BOM so
// spreadsheets open umlauts correctly (research R19). Text that looks like a
// formula is neutralized.
func WriteCSV(w io.Writer, h History) error {
	if _, err := io.WriteString(w, "\uFEFF"); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(CSVHeader); err != nil {
		return err
	}
	if h.Instance.Name != "" {
		rec := make([]string, len(CSVHeader))
		rec[0], rec[3] = "instance", h.Instance.Name
		if err := cw.Write(neutralize(rec)); err != nil {
			return err
		}
	}
	v := h.Vehicle
	meters := map[int64]string{}
	for _, m := range v.HourMeters {
		meters[m.ID] = m.Name
	}
	base := slices.Index(CSVHeader, "hsn")
	vehicleRow := make([]string, len(CSVHeader))
	vehicleRow[len(vehicleRow)-1] = v.VehicleType
	vehicleRow[0], vehicleRow[2], vehicleRow[3], vehicleRow[4], vehicleRow[6] =
		"vehicle", kmCol(v.CurrentOdometerKm, v.HasOdometer), strings.TrimSpace(v.Make+" "+v.Model), v.ChassisCode, v.EngineCode
	copy(vehicleRow[base:], []string{strOr(v.HSN), strOr(v.TSN), intOr(v.DisplacementCC), intOr(v.PowerKW), strOr(v.FuelType)})
	if len(v.HourMeters) > 0 {
		m := v.HourMeters[0]
		copy(vehicleRow[base+vehicleCols:], hoursCols(meters, []HistoryReading{{HourMeterID: m.ID, Hours: m.CurrentHours}}))
	}
	if err := cw.Write(neutralize(vehicleRow)); err != nil {
		return err
	}
	// Line-item rows leave the vehicle columns empty, carry their hours and
	// leave the vehicle type empty.
	write := func(rec []string, readings []HistoryReading) error {
		rec = append(rec, make([]string, vehicleCols)...)
		return cw.Write(neutralize(append(append(rec, hoursCols(meters, readings)...), "")))
	}
	for _, s := range h.Services {
		km := kmCol(s.OdometerKm, s.OdometerRecorded)
		for _, it := range s.Items {
			q, unit := qty(it.QuantityMl, it.QuantityUnit)
			if err := write([]string{"service", s.Date, km, "", it.Category, it.Brand, it.Specification,
				q, unit, it.OEMPartNumber, it.AftermarketPartNumber, money(it.CostCents), it.CostType, "", s.Notes}, s.MeterReadings); err != nil {
				return err
			}
		}
		if s.ExtraCostCents != nil && *s.ExtraCostCents > 0 {
			if err := write([]string{"service_extra", s.Date, km, s.ExtraCostLabel, "", "", "",
				"", "", "", "", money(s.ExtraCostCents), "other", "", ""}, s.MeterReadings); err != nil {
				return err
			}
		}
	}
	for _, r := range h.Repairs {
		km := kmCol(r.OdometerKm, r.OdometerRecorded)
		if len(r.Parts) == 0 {
			if err := write([]string{"repair", r.CompletedOn, km, r.Title, "", "", "", "", "", "", "", "", "", "completed", r.Procedure}, r.MeterReadings); err != nil {
				return err
			}
			continue
		}
		for _, p := range r.Parts {
			var cost *int64
			if p.UnitCostCents != nil {
				v := *p.UnitCostCents * p.Quantity
				cost = &v
			}
			if err := write([]string{"repair_part", r.CompletedOn, km, r.Title, p.Name, p.AftermarketBrand, "",
				strconv.FormatInt(p.Quantity, 10), "pcs", p.OEMPartNumber, p.AftermarketPartNumber, money(cost), p.CostType, p.Status, r.Procedure}, r.MeterReadings); err != nil {
				return err
			}
		}
	}
	cw.Flush()
	return cw.Error()
}
