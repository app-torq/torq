package reports

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/app-torq/torq/internal/i18n"
	"github.com/app-torq/torq/internal/repairs"
	"github.com/app-torq/torq/internal/servicelog"
	"github.com/app-torq/torq/internal/testutil"
	"github.com/app-torq/torq/internal/vehicles"
)

type fx struct {
	ctx context.Context
	r   *Service
	veh *vehicles.Service
	svc *servicelog.Service
	rep *repairs.Service
	vid int64
}

func i64(v int64) *int64 { return &v }

func setup(t testing.TB) *fx {
	t.Helper()
	d := testutil.NewDB(t)
	clk := testutil.FixedClock(t, "2025-11-01")
	f := &fx{ctx: context.Background()}
	f.veh = &vehicles.Service{DB: d, Clock: clk}
	f.svc = &servicelog.Service{DB: d, Clock: clk, Vehicles: f.veh}
	f.rep = &repairs.Service{DB: d, Clock: clk, Vehicles: f.veh}
	f.r = &Service{DB: d, Vehicles: f.veh, ServiceLog: f.svc, Repairs: f.rep}
	v, err := f.veh.Create(f.ctx, vehicles.Profile{Make: "Mercedes-Benz", Model: "C 220 CDI", ChassisCode: "S203",
		HSN: ptr("0710"), TSN: ptr("abc"), DisplacementCC: i64(2148), PowerKW: i64(110), FuelType: ptr("diesel")}, 204000, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.vid = v.ID
	// Its initial reading is dated 2025-11-01, outside the 2026 report range.
	clk.At = clk.At.AddDate(1, 1, 30)
	return f
}

// seed2026: services €420 (incl. €20 other), completed repairs €1,180, readings 205,000 → 225,000 km.
func (f *fx) seed2026(t testing.TB) {
	t.Helper()
	mustLog := func(date string, km int64, extra int64, items ...servicelog.ItemInput) {
		if _, _, err := f.svc.Create(f.ctx, servicelog.Input{VehicleID: f.vid, ServiceDate: date, OdometerKm: km, ExtraCostCents: extra, ExtraCostLabel: "Shop supplies", Notes: "private, \"quoted\" note", Items: items}, 0); err != nil {
			t.Fatal(err)
		}
	}
	mustLog("2026-01-10", 205000, 2000, servicelog.ItemInput{CategoryID: 1, Brand: "Mobil 1", QuantityMl: i64(6500), QuantityUnit: "L", CostCents: i64(9000)})
	mustLog("2026-06-01", 215000, 0, servicelog.ItemInput{CategoryID: 6, CostCents: i64(31000)})
	// Outside the range: must not count.
	mustLog("2025-12-01", 204000, 0, servicelog.ItemInput{CategoryID: 1, CostCents: i64(99999)})

	r, err := f.rep.Create(f.ctx, repairs.Input{VehicleID: f.vid, Title: "Injektor-Dichtungen, \"Kupfer\""}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.rep.CreatePart(f.ctx, r.ID, repairs.PartInput{Name: "Dichtring, Öl", Quantity: 4, UnitCostCents: i64(25000)}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.rep.CreatePart(f.ctx, r.ID, repairs.PartInput{Name: "Kupferpaste", Quantity: 1, UnitCostCents: i64(18000), CostType: "fluids"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.rep.SetStatus(f.ctx, r.ID, repairs.StatusCompleted, "2026-11-20", i64(225000)); err != nil {
		t.Fatal(err)
	}
	// Open project: must not count.
	open, _ := f.rep.Create(f.ctx, repairs.Input{VehicleID: f.vid, Title: "Open"}, 0)
	if _, err := f.rep.CreatePart(f.ctx, open.ID, repairs.PartInput{Name: "x", UnitCostCents: i64(50000)}); err != nil {
		t.Fatal(err)
	}
}

func TestCostReportExact(t *testing.T) {
	f := setup(t)
	f.seed2026(t)
	rep, err := f.r.CostReport(f.ctx, f.vid, "2026-01-01", "2026-12-31")
	if err != nil {
		t.Fatal(err)
	}
	if rep.TotalCents != 160000 || rep.RoutineServiceCents != 42000 || rep.SelfRepairCents != 118000 {
		t.Fatalf("totals %+v", rep.Costs)
	}
	if rep.PartsCents != 100000+31000 || rep.FluidsCents != 9000+18000 || rep.OtherCents != 2000 {
		t.Fatalf("types parts=%d fluids=%d other=%d", rep.PartsCents, rep.FluidsCents, rep.OtherCents)
	}
	if rep.DistanceKm == nil || *rep.DistanceKm != 20000 || *rep.CostPerKmCents() != 8 {
		t.Fatalf("distance %v cpk %v", rep.DistanceKm, rep.CostPerKmCents())
	}

	// A range with a single reading has no cost per km.
	one, err := f.r.CostReport(f.ctx, f.vid, "2026-06-01", "2026-06-30")
	if err != nil {
		t.Fatal(err)
	}
	if one.DistanceKm != nil || one.CostPerKmCents() != nil {
		t.Fatal("single reading must not yield a distance")
	}

	// Garage-wide equals the sum of vehicles.
	g, err := f.r.CostReport(f.ctx, 0, "2026-01-01", "2026-12-31")
	if err != nil {
		t.Fatal(err)
	}
	if g.TotalCents != 160000 || len(g.ByVehicle) != 1 || g.ByVehicle[0].TotalCents != 160000 {
		t.Fatalf("garage %+v", g)
	}

	// Reopening a repair removes it from the report.
	reps, _, _ := f.rep.List(f.ctx, repairs.Filter{Statuses: []string{repairs.StatusCompleted}, Limit: 10})
	if _, err := f.rep.SetStatus(f.ctx, reps[0].ID, repairs.StatusInProgress, "", nil); err != nil {
		t.Fatal(err)
	}
	rep, _ = f.r.CostReport(f.ctx, f.vid, "2026-01-01", "2026-12-31")
	if rep.SelfRepairCents != 0 {
		t.Fatalf("reopened repair counted: %d", rep.SelfRepairCents)
	}
	if _, err := f.r.CostReport(f.ctx, f.vid, "2026-12-31", "2026-01-01"); err == nil {
		t.Fatal("inverted range accepted")
	}
}

func TestTimeline(t *testing.T) {
	f := setup(t)
	f.seed2026(t)
	items, total, err := f.r.Timeline(f.ctx, f.vid, "", "", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || items[0].Kind != KindRepair || items[0].Date != "2026-11-20" || items[len(items)-1].Date != "2025-12-01" {
		t.Fatalf("timeline %+v", items)
	}
	page, _, _ := f.r.Timeline(f.ctx, f.vid, "", "", 2, 2)
	if len(page) != 2 || page[0].Date != "2026-01-10" {
		t.Fatalf("page %+v", page)
	}
}

func ptr[T any](v T) *T { return &v }

func TestHistoryExport(t *testing.T) {
	f := setup(t)
	f.seed2026(t)
	h, err := f.r.History(f.ctx, f.vid, ExportOptions{IncludeCosts: true, IncludeNotes: true, Locale: i18n.Locale{Lang: "de"}})
	if err != nil {
		t.Fatal(err)
	}
	if h.Schema != "torq.vehicle-history/v1" || len(h.Services) != 3 || len(h.Repairs) != 1 || len(h.Repairs[0].Parts) != 2 {
		t.Fatalf("history %+v", h)
	}
	if h.Services[0].Items[0].Category == "" || h.Services[len(h.Services)-1].Items[0].Category != "Motoröl & Filter" {
		t.Fatalf("localized category: %+v", h.Services[len(h.Services)-1].Items[0])
	}

	var buf bytes.Buffer
	if err := WriteCSV(&buf, h); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("\xEF\xBB\xBF")) {
		t.Fatal("missing UTF-8 BOM")
	}
	rows, err := csv.NewReader(bytes.NewReader(buf.Bytes()[3:])).ReadAll()
	if err != nil {
		t.Fatalf("not RFC 4180: %v", err)
	}
	// The hour meter columns (#24) and the vehicle type (#28) are appended, so earlier columns keep their positions.
	if strings.Join(rows[0], ",") != "record_type,date,odometer_km,record_title,category_or_part,brand,specification,quantity,unit,oem_part_number,aftermarket_part_number,cost,cost_type,status,notes,hsn,tsn,displacement_cc,power_kw,fuel_type,hour_meter,hours,vehicle_type" {
		t.Fatalf("header %v", rows[0])
	}
	// FR-027: the leading vehicle record carries the extended profile; JSON has the fields too.
	if v := rows[1]; v[0] != "vehicle" || v[3] != "Mercedes-Benz C 220 CDI" || v[15] != "0710" || v[16] != "ABC" || v[17] != "2148" || v[18] != "110" || v[19] != "diesel" || v[22] != "car" {
		t.Fatalf("vehicle row %v", v)
	}
	if hv := h.Vehicle; hv.HSN == nil || *hv.HSN != "0710" || hv.PowerKW == nil || *hv.PowerKW != 110 || hv.FuelType == nil || *hv.FuelType != "diesel" {
		t.Fatalf("json vehicle %+v", hv)
	}
	for _, r := range rows[2:] {
		if len(r) != len(CSVHeader) || r[15] != "" || r[19] != "" || r[22] != "" {
			t.Fatalf("line item carries vehicle columns: %v", r)
		}
	}
	var found bool
	for _, r := range rows[1:] {
		if r[0] == "repair_part" && r[4] == "Dichtring, Öl" && r[11] == "1000.00" && r[3] == `Injektor-Dichtungen, "Kupfer"` {
			found = true
		}
		if r[0] == "service" && r[7] == "6.5" && r[8] == "L" && r[14] != `private, "quoted" note` {
			t.Fatalf("notes round-trip %q", r[14])
		}
	}
	if !found {
		t.Fatalf("repair part row missing: %v", rows)
	}

	// Without costs and notes.
	h, _ = f.r.History(f.ctx, f.vid, ExportOptions{})
	buf.Reset()
	_ = WriteCSV(&buf, h)
	rows, _ = csv.NewReader(bytes.NewReader(buf.Bytes()[3:])).ReadAll()
	for _, r := range rows[2:] {
		if r[11] != "" || r[14] != "" {
			t.Fatalf("costs/notes leaked: %v", r)
		}
	}
}

func TestExportPerformance(t *testing.T) {
	f := setup(t)
	km := int64(205000)
	for i := 0; i < 500; i++ {
		km += 100
		date := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i*3).Format("2006-01-02")
		if _, _, err := f.svc.Create(f.ctx, servicelog.Input{VehicleID: f.vid, ServiceDate: date, OdometerKm: km,
			Items: []servicelog.ItemInput{{CategoryID: 1, CostCents: i64(5000)}, {CategoryID: 6, CostCents: i64(2000)}}}, 0); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	h, err := f.r.History(f.ctx, f.vid, ExportOptions{IncludeCosts: true, IncludeNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := WriteCSV(&buf, h); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 10*time.Second || len(h.Services) != 500 {
		t.Fatalf("500-record export took %v (%d services)", d, len(h.Services))
	}
}

func TestCSVNeutralizesFormulas(t *testing.T) {
	h := History{
		Vehicle: HistoryVehicle{Make: "=HYPERLINK(\"http://x\")", Model: "C 220"},
		Repairs: []HistoryRepair{{CompletedOn: "2026-10-30", Title: "@SUM(A1)", Procedure: "-2+3", OdometerRecorded: true}},
	}
	var buf bytes.Buffer
	if err := WriteCSV(&buf, h); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(bytes.NewReader(buf.Bytes()[3:])).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if v := rows[1][3]; v != `'=HYPERLINK("http://x") C 220` {
		t.Fatalf("vehicle %q", v)
	}
	if r := rows[2]; r[3] != "'@SUM(A1)" || r[14] != "'-2+3" || r[2] != "0" {
		t.Fatalf("repair %v", r)
	}
	if got := neutralize([]string{"-12.50", "+5", "a=b"}); strings.Join(got, "|") != "-12.50|+5|a=b" {
		t.Fatalf("numbers and inner signs must stay: %v", got)
	}
}

// TestHoursReports covers a vehicle tracked by hours only (#24): cost per
// operating hour (€840 over 1,250 → 1,450 h = €4.20/h), no distance, and
// hours in the history and CSV.
func TestHoursReports(t *testing.T) {
	f := setup(t)
	f.seed2026(t)
	tractor, err := f.veh.Create(f.ctx, vehicles.Profile{Make: "Deutz", Model: "D 6206", NoOdometer: true,
		InitialMeter: &vehicles.MeterInput{Name: "Engine hours", HoursTenths: 12500, ReadingDate: "2026-01-02"}}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	meters, err := f.veh.Meters(f.ctx, []int64{tractor.ID})
	if err != nil {
		t.Fatal(err)
	}
	meter := meters[tractor.ID][0].ID
	if _, _, err := f.svc.Create(f.ctx, servicelog.Input{VehicleID: tractor.ID, ServiceDate: "2026-06-01", NoOdometerKm: true,
		MeterReadings: []vehicles.HoursReading{{MeterID: meter, HoursTenths: 14500}},
		Items:         []servicelog.ItemInput{{CategoryID: 1, CostCents: i64(84000)}}}, 0); err != nil {
		t.Fatal(err)
	}

	rep, err := f.r.CostReport(f.ctx, tractor.ID, "2026-01-01", "2026-12-31")
	if err != nil {
		t.Fatal(err)
	}
	if rep.DistanceKm != nil || rep.HoursTenths == nil || *rep.HoursTenths != 2000 || rep.CostPerHourCents() == nil || *rep.CostPerHourCents() != 420 {
		t.Fatalf("tractor report %+v", rep)
	}
	// The garage's distance and cost per km ignore the tractor.
	all, err := f.r.CostReport(f.ctx, 0, "2026-01-01", "2026-12-31")
	if err != nil {
		t.Fatal(err)
	}
	if all.DistanceKm == nil || *all.DistanceKm != 20000 || all.HoursTenths != nil {
		t.Fatalf("garage report %+v", all)
	}
	// One reading in the period: not enough data.
	single, err := f.r.CostReport(f.ctx, tractor.ID, "2026-05-01", "2026-12-31")
	if err != nil {
		t.Fatal(err)
	}
	if single.HoursTenths != nil || single.CostPerHourCents() != nil {
		t.Fatalf("single reading %+v", single)
	}

	h, err := f.r.History(f.ctx, tractor.ID, ExportOptions{IncludeCosts: true, Locale: i18n.Default})
	if err != nil {
		t.Fatal(err)
	}
	if h.Vehicle.HasOdometer || len(h.Vehicle.HourMeters) != 1 || h.Vehicle.HourMeters[0].CurrentHours != 1450 {
		t.Fatalf("history vehicle %+v", h.Vehicle)
	}
	if s := h.Services[0]; s.OdometerRecorded || len(s.MeterReadings) != 1 || s.MeterReadings[0].Hours != 1450 {
		t.Fatalf("history service %+v", s)
	}
	var buf bytes.Buffer
	if err := WriteCSV(&buf, h); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(bytes.NewReader(buf.Bytes()[3:])).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	// hour_meter and hours are columns 20 and 21.
	if v := rows[1]; v[2] != "" || v[20] != "Engine hours" || v[21] != "1450" {
		t.Fatalf("vehicle row %v", v)
	}
	if s := rows[2]; s[0] != "service" || s[2] != "" || s[20] != "Engine hours" || s[21] != "1450" {
		t.Fatalf("service row %v", s)
	}

	items, _, err := f.r.Timeline(f.ctx, tractor.ID, "", "", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].OdometerRecorded || len(items[0].MeterReadings) != 1 {
		t.Fatalf("timeline %+v", items)
	}
}
