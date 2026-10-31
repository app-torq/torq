package reports

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/marekbeckmann/torq/internal/i18n"
	"github.com/marekbeckmann/torq/internal/repairs"
	"github.com/marekbeckmann/torq/internal/servicelog"
	"github.com/marekbeckmann/torq/internal/testutil"
	"github.com/marekbeckmann/torq/internal/vehicles"
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
	v, err := f.veh.Create(f.ctx, vehicles.Profile{Make: "Mercedes-Benz", Model: "C 220 CDI", ChassisCode: "S203"}, 204000, 0)
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
	if strings.Join(rows[0], ",") != "record_type,date,odometer_km,record_title,category_or_part,brand,specification,quantity,unit,oem_part_number,aftermarket_part_number,cost,cost_type,status,notes" {
		t.Fatalf("header %v", rows[0])
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
	for _, r := range rows[1:] {
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
