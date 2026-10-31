package reports

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/marekbeckmann/torq/internal/i18n"
	"github.com/marekbeckmann/torq/internal/repairs"
	"github.com/marekbeckmann/torq/internal/servicelog"
	"github.com/marekbeckmann/torq/internal/store"
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
	Vehicle  HistoryVehicle   `json:"vehicle"`
	Specs    map[string]any   `json:"specs"`
	Entries  []HistoryEntry   `json:"spec_entries"`
	Services []HistoryService `json:"service_logs"`
	Repairs  []HistoryRepair  `json:"repairs"`
}

type HistoryVehicle struct {
	ID                int64   `json:"id"`
	Make              string  `json:"make"`
	Model             string  `json:"model"`
	ChassisCode       string  `json:"chassis_code"`
	ModelYear         *int64  `json:"model_year"`
	EngineCode        string  `json:"engine_code"`
	VIN               *string `json:"vin"`
	LicensePlate      *string `json:"license_plate"`
	CurrentOdometerKm int64   `json:"current_odometer_km"`
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
	ID             int64                `json:"id"`
	Date           string               `json:"service_date"`
	OdometerKm     int64                `json:"odometer_km"`
	Notes          string               `json:"notes,omitempty"`
	ExtraCostCents *int64               `json:"extra_cost_cents,omitempty"`
	ExtraCostLabel string               `json:"extra_cost_label,omitempty"`
	TotalCents     *int64               `json:"total_cents,omitempty"`
	Items          []HistoryServiceItem `json:"items"`
	Attachments    []HistoryAttachment  `json:"attachments"`
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
	ID           int64               `json:"id"`
	Title        string              `json:"title"`
	StartedOn    *string             `json:"started_on"`
	CompletedOn  string              `json:"completed_on"`
	OdometerKm   int64               `json:"completion_odometer_km"`
	Symptom      string              `json:"symptom"`
	RootCause    string              `json:"root_cause"`
	Procedure    string              `json:"procedure_notes,omitempty"`
	SpecialTools string              `json:"special_tools"`
	FaultCodes   []string            `json:"fault_codes"`
	TorqueSpecs  []HistoryTorque     `json:"torque_specs"`
	Parts        []HistoryPart       `json:"parts"`
	PartsTotal   *int64              `json:"parts_total_cents,omitempty"`
	Attachments  []HistoryAttachment `json:"attachments"`
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
	h := History{
		Schema: "torq.vehicle-history/v1",
		Vehicle: HistoryVehicle{ID: v.ID, Make: v.Make, Model: v.Model, ChassisCode: v.ChassisCode, ModelYear: ptrI(v.ModelYear),
			EngineCode: v.EngineCode, VIN: ptrS(v.Vin), LicensePlate: ptrS(v.LicensePlate), CurrentOdometerKm: v.CurrentOdometerKm},
		Specs: map[string]any{
			"engine_oil_viscosity": spec.EngineOilViscosity, "engine_oil_spec": spec.EngineOilSpec, "engine_oil_capacity_ml": ptrI(spec.EngineOilCapacityMl),
			"transmission_fluid_spec": spec.TransmissionFluidSpec, "transmission_fluid_capacity_ml": ptrI(spec.TransmissionFluidCapacityMl),
			"differential_fluid_spec": spec.DifferentialFluidSpec, "differential_fluid_capacity_ml": ptrI(spec.DifferentialFluidCapacityMl),
			"brake_fluid_spec": spec.BrakeFluidSpec, "brake_fluid_capacity_ml": ptrI(spec.BrakeFluidCapacityMl),
			"coolant_spec": spec.CoolantSpec, "coolant_capacity_ml": ptrI(spec.CoolantCapacityMl),
			"wheel_lug_torque_nm": ptrF(spec.WheelLugTorqueNm), "tire_pressure_front_bar": ptrF(spec.TirePressureFrontBar),
			"tire_pressure_rear_bar": ptrF(spec.TirePressureRearBar),
		},
		Entries: []HistoryEntry{}, Services: []HistoryService{}, Repairs: []HistoryRepair{},
	}
	for _, e := range entries {
		h.Entries = append(h.Entries, HistoryEntry{Label: e.Label, Value: e.Value})
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
		hs := HistoryService{ID: l.ID, Date: l.ServiceDate, OdometerKm: l.OdometerKm, Items: []HistoryServiceItem{}, Attachments: attachmentsOf(atts)}
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
			OdometerKm: r.CompletionOdometerKm.Int64, Symptom: r.Symptom, RootCause: r.RootCause, SpecialTools: r.SpecialTools,
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
	rows, err := q.ListCategories(ctx, 1)
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

// CSVHeader is the fixed column order of the CSV export.
var CSVHeader = []string{"record_type", "date", "odometer_km", "record_title", "category_or_part", "brand", "specification",
	"quantity", "unit", "oem_part_number", "aftermarket_part_number", "cost", "cost_type", "status", "notes"}

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

func qty(ml *int64, unit *string) (string, string) {
	if ml == nil || unit == nil {
		return "", ""
	}
	if *unit == "L" {
		return strconv.FormatFloat(float64(*ml)/1000, 'f', -1, 64), "L"
	}
	return strconv.FormatInt(*ml, 10), "mL"
}

// WriteCSV writes the history as one flat RFC 4180 file, UTF-8 with BOM so
// spreadsheets open umlauts correctly (research R19).
func WriteCSV(w io.Writer, h History) error {
	if _, err := io.WriteString(w, "\uFEFF"); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(CSVHeader); err != nil {
		return err
	}
	for _, s := range h.Services {
		for _, it := range s.Items {
			q, unit := qty(it.QuantityMl, it.QuantityUnit)
			if err := cw.Write([]string{"service", s.Date, strconv.FormatInt(s.OdometerKm, 10), "", it.Category, it.Brand, it.Specification,
				q, unit, it.OEMPartNumber, it.AftermarketPartNumber, money(it.CostCents), it.CostType, "", s.Notes}); err != nil {
				return err
			}
		}
		if s.ExtraCostCents != nil && *s.ExtraCostCents > 0 {
			if err := cw.Write([]string{"service_extra", s.Date, strconv.FormatInt(s.OdometerKm, 10), s.ExtraCostLabel, "", "", "",
				"", "", "", "", money(s.ExtraCostCents), "other", "", ""}); err != nil {
				return err
			}
		}
	}
	for _, r := range h.Repairs {
		km := strconv.FormatInt(r.OdometerKm, 10)
		if len(r.Parts) == 0 {
			if err := cw.Write([]string{"repair", r.CompletedOn, km, r.Title, "", "", "", "", "", "", "", "", "", "completed", r.Procedure}); err != nil {
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
			if err := cw.Write([]string{"repair_part", r.CompletedOn, km, r.Title, p.Name, p.AftermarketBrand, "",
				strconv.FormatInt(p.Quantity, 10), "pcs", p.OEMPartNumber, p.AftermarketPartNumber, money(cost), p.CostType, p.Status, r.Procedure}); err != nil {
				return err
			}
		}
	}
	cw.Flush()
	return cw.Error()
}
