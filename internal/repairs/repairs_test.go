package repairs

import (
	"context"
	"testing"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/testutil"
	"github.com/app-torq/torq/internal/vehicles"
)

func setup(t *testing.T) (*Service, *vehicles.Service, int64, context.Context) {
	t.Helper()
	d := testutil.NewDB(t)
	clk := testutil.FixedClock(t, "2026-10-30")
	veh := &vehicles.Service{DB: d, Clock: clk}
	v, err := veh.Create(context.Background(), vehicles.Profile{Make: "Mercedes-Benz", Model: "C 220 CDI"}, 215200, 0)
	if err != nil {
		t.Fatal(err)
	}
	return &Service{DB: d, Clock: clk, Vehicles: veh}, veh, v.ID, context.Background()
}

func i64(v int64) *int64 { return &v }

func TestWorkflowAndCompletion(t *testing.T) {
	s, veh, vid, ctx := setup(t)
	r, err := s.Create(ctx, Input{VehicleID: vid, Title: "Injector seal replacement", Symptom: "rough idle cold start", FaultCodes: []string{"P0201", "p0300", "P0201", " "}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusPlanned || len(r.FaultCodes) != 2 || r.FaultCodes[1] != "P0300" {
		t.Fatalf("created %+v", r)
	}

	r, err = s.SetStatus(ctx, r.ID, StatusInProgress, "", nil)
	if err != nil || r.StartedOn.String != "2026-10-30" {
		t.Fatalf("in progress: %v %v", r.StartedOn, err)
	}

	if _, err := s.SetStatus(ctx, r.ID, StatusCompleted, "2026-10-30", nil); err == nil {
		t.Fatal("completing without odometer must fail")
	} else if e, _ := apperr.As(err); e.Fields["completion_odometer_km"] != "validation.required" {
		t.Fatalf("fields %v", e.Fields)
	}

	if _, err = s.SetStatus(ctx, r.ID, StatusCompleted, "2026-10-30", i64(216000)); err != nil {
		t.Fatal(err)
	}
	v, _ := veh.Get(ctx, vid)
	if v.CurrentOdometerKm != 216000 {
		t.Fatalf("odometer %d", v.CurrentOdometerKm)
	}
	readings, _ := veh.Readings(ctx, vid, "", "")
	if readings[0].Source != vehicles.SourceRepairCompletion {
		t.Fatalf("latest reading %+v", readings[0])
	}

	// Reopen: reading removed, completion values kept.
	r, err = s.SetStatus(ctx, r.ID, StatusInProgress, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !r.CompletedOn.Valid || r.CompletionOdometerKm.Int64 != 216000 {
		t.Fatalf("completion values lost: %+v", r.RepairLog)
	}
	readings, _ = veh.Readings(ctx, vid, "", "")
	for _, rd := range readings {
		if rd.Source == vehicles.SourceRepairCompletion {
			t.Fatal("completion reading not removed on reopen")
		}
	}
}

func TestBOMTotals(t *testing.T) {
	s, _, vid, ctx := setup(t)
	r, err := s.Create(ctx, Input{VehicleID: vid, Title: "Seals"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	parts := []PartInput{
		{Name: "Injector seal", Quantity: 4, UnitCostCents: i64(295)},
		{Name: "Clamp bolt", Quantity: 4, UnitCostCents: i64(189), Status: "ordered"},
		{Name: "Return line", Quantity: 1, UnitCostCents: i64(1999), Status: "in_stock", SupplierURL: "https://shop.example/part"},
		{Name: "Copper grease", CostType: "fluids", Status: "installed"},
	}
	for _, p := range parts {
		if _, err := s.CreatePart(ctx, r.ID, p); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Get(ctx, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Totals.TotalCents != 4*295+4*189+1999 {
		t.Fatalf("total %d", got.Totals.TotalCents)
	}
	c := got.Totals.StatusCounts
	if c["needed"] != 1 || c["ordered"] != 1 || c["in_stock"] != 1 || c["installed"] != 1 {
		t.Fatalf("counts %v", c)
	}
	if _, err := s.CreatePart(ctx, r.ID, PartInput{Name: "x", SupplierURL: "javascript:alert(1)"}); err == nil {
		t.Fatal("non-http supplier URL accepted")
	}
	if _, err := s.CreatePart(ctx, r.ID, PartInput{Name: "x", Quantity: -1}); err == nil {
		t.Fatal("negative quantity accepted")
	}
	ids := []int64{got.Parts[3].ID, got.Parts[2].ID, got.Parts[1].ID, got.Parts[0].ID}
	reordered, err := s.ReorderParts(ctx, r.ID, ids)
	if err != nil || reordered[0].Name != "Copper grease" {
		t.Fatalf("reorder %v %v", reordered, err)
	}
}

func TestSearchAndFilter(t *testing.T) {
	s, _, vid, ctx := setup(t)
	a, _ := s.Create(ctx, Input{VehicleID: vid, Title: "Injector seals", FaultCodes: []string{"P0201"}}, 0)
	if _, err := s.CreatePart(ctx, a.ID, PartInput{Name: "Seal", OEMPartNumber: "A6110170060"}); err != nil {
		t.Fatal(err)
	}
	b, _ := s.Create(ctx, Input{VehicleID: vid, Title: "Swirl flaps"}, 0)
	if _, err := s.SetStatus(ctx, b.ID, StatusWaitingOnParts, "", nil); err != nil {
		t.Fatal(err)
	}
	for q, want := range map[string]int64{"P0201": 1, "A6110170060": 1, "swirl": 1, "": 2, "nothing": 0} {
		_, total, err := s.List(ctx, Filter{Q: q, Limit: 50})
		if err != nil || total != want {
			t.Fatalf("q=%q: total %d err %v", q, total, err)
		}
	}
	rows, total, err := s.List(ctx, Filter{Statuses: []string{StatusPlanned, StatusWaitingOnParts}, Limit: 50})
	if err != nil || total != 2 || rows[0].Status != StatusWaitingOnParts {
		t.Fatalf("status filter: %d %v", total, err)
	}
	if _, _, err := s.List(ctx, Filter{Statuses: []string{"bogus"}, Limit: 50}); err == nil {
		t.Fatal("invalid status accepted")
	}
}

func TestReopenWithoutStartDate(t *testing.T) {
	s, _, vid, ctx := setup(t)
	r, err := s.Create(ctx, Input{VehicleID: vid, Title: "Quick fix"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetStatus(ctx, r.ID, StatusCompleted, "2026-10-01", i64(215300)); err != nil {
		t.Fatal(err)
	}
	r, err = s.SetStatus(ctx, r.ID, StatusInProgress, "", nil)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	if r.StartedOn.String != "2026-10-01" {
		t.Fatalf("started_on %q, want the retained completion date", r.StartedOn.String)
	}
}
