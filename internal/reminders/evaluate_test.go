package reminders

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/servicelog"
	"github.com/app-torq/torq/internal/store"
	"github.com/app-torq/torq/internal/testutil"
	"github.com/app-torq/torq/internal/vehicles"
)

type fixture struct {
	ctx         context.Context
	clk         *clock.Fixed
	veh         *vehicles.Service
	rem         *Service
	svc         *servicelog.Service
	vehicleID   int64
	transitions []Transition
}

func newFixture(t *testing.T, km int64) *fixture {
	t.Helper()
	d := testutil.NewDB(t)
	f := &fixture{ctx: context.Background(), clk: testutil.FixedClock(t, "2026-10-30")}
	f.veh = &vehicles.Service{DB: d, Clock: f.clk}
	f.rem = &Service{DB: d, Clock: f.clk, Vehicles: f.veh}
	f.rem.OnTransitions = func(_ context.Context, _ *sql.Tx, ts []Transition) error {
		f.transitions = append(f.transitions, ts...)
		return nil
	}
	f.veh.AfterOdometerChange = f.rem.EvaluateVehicle
	f.svc = &servicelog.Service{DB: d, Clock: f.clk, Vehicles: f.veh}
	v, err := f.veh.Create(f.ctx, vehicles.Profile{Make: "Mercedes-Benz", Model: "C 220 CDI"}, km, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.vehicleID = v.ID
	return f
}

func i64(v int64) *int64 { return &v }

const engineOilCategory = 1 // first seeded built-in

func (f *fixture) oilReminder(t *testing.T, start string, startKm int64) store.Reminder {
	t.Helper()
	cat := int64(engineOilCategory)
	r, err := f.rem.Create(f.ctx, Input{
		VehicleID: f.vehicleID, ServiceCategoryID: &cat, IntervalKm: i64(10000), IntervalMonths: i64(12),
		StartDate: start, StartOdometerKm: &startKm,
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (f *fixture) odometer(t *testing.T, km int64) {
	t.Helper()
	if _, err := f.veh.QuickUpdate(f.ctx, f.vehicleID, vehicles.QuickUpdateInput{OdometerKm: km, ConfirmDecrease: true, ConfirmLargeJump: true}); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) reload(t *testing.T, id int64) store.Reminder {
	t.Helper()
	r, err := f.rem.Get(f.ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (f *fixture) logOil(t *testing.T, date string, km int64) servicelog.Log {
	t.Helper()
	l, _, err := f.svc.Create(f.ctx, servicelog.Input{
		VehicleID: f.vehicleID, ServiceDate: date, OdometerKm: km,
		Items: []servicelog.ItemInput{{CategoryID: engineOilCategory, QuantityMl: i64(6500), QuantityUnit: "L", CostCents: i64(8950)}},
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestTransitionsAndIdempotency(t *testing.T) {
	f := newFixture(t, 214500)
	r := f.oilReminder(t, "2026-08-01", 205500) // due at 215,500 km / 2027-08-01

	f.odometer(t, 214600)
	if got := f.reload(t, r.ID); got.State != "due_soon" || got.StateTrigger.String != "km" {
		t.Fatalf("state %s/%s", got.State, got.StateTrigger.String)
	}
	if len(f.transitions) != 1 || f.transitions[0].To != "due_soon" {
		t.Fatalf("transitions %+v", f.transitions)
	}

	// Re-evaluating without crossing a threshold must not notify again.
	f.odometer(t, 214700)
	if err := f.rem.EvaluateAll(f.ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.transitions) != 1 {
		t.Fatalf("duplicate transitions %+v", f.transitions)
	}

	f.odometer(t, 215600)
	if got := f.reload(t, r.ID); got.State != "overdue" {
		t.Fatalf("state %s", got.State)
	}
	if len(f.transitions) != 2 || f.transitions[1].To != "overdue" {
		t.Fatalf("transitions %+v", f.transitions)
	}

	// Logging the service resets to upcoming; re-entering due_soon notifies again.
	f.logOil(t, "2026-10-30", 215600)
	got := f.reload(t, r.ID)
	if got.State != "upcoming" || got.NextDueKm.Int64 != 225600 || got.NextDueDate.String != "2027-10-30" || got.LastNotifiedState != "upcoming" {
		t.Fatalf("after service: %+v", got)
	}
	f.odometer(t, 224700)
	if len(f.transitions) != 3 || f.transitions[2].To != "due_soon" {
		t.Fatalf("re-entry transitions %+v", f.transitions)
	}
}

func TestLargeJumpSkipsDueSoonAndDowngradeIsSilent(t *testing.T) {
	f := newFixture(t, 200000)
	r := f.oilReminder(t, "2026-08-01", 200000)
	f.odometer(t, 211000)
	if len(f.transitions) != 1 || f.transitions[0].To != "overdue" {
		t.Fatalf("transitions %+v", f.transitions)
	}
	// Typo fix back into the due-soon window: no notification.
	f.odometer(t, 209500)
	got := f.reload(t, r.ID)
	if got.State != "due_soon" || got.LastNotifiedState != "due_soon" || len(f.transitions) != 1 {
		t.Fatalf("downgrade: %+v, transitions %d", got, len(f.transitions))
	}
}

func TestBaselineUsesLatestServiceAndFallsBack(t *testing.T) {
	f := newFixture(t, 214500)
	r := f.oilReminder(t, "2026-08-01", 205500)
	latest := f.logOil(t, "2026-10-15", 215000)
	if got := f.reload(t, r.ID); got.NextDueKm.Int64 != 225000 || got.NextDueDate.String != "2027-10-15" {
		t.Fatalf("after service: %d %s", got.NextDueKm.Int64, got.NextDueDate.String)
	}
	// Back-filled older service does not move the due point.
	f.logOil(t, "2025-10-01", 200000)
	if got := f.reload(t, r.ID); got.NextDueKm.Int64 != 225000 {
		t.Fatalf("back-fill changed due km to %d", got.NextDueKm.Int64)
	}
	cur, _ := f.veh.Get(f.ctx, f.vehicleID)
	if cur.CurrentOdometerKm != 215000 {
		t.Fatalf("back-fill lowered odometer to %d", cur.CurrentOdometerKm)
	}
	// Deleting the latest falls back to the previous service.
	if _, err := f.svc.Delete(f.ctx, latest.ID); err != nil {
		t.Fatal(err)
	}
	if got := f.reload(t, r.ID); got.NextDueKm.Int64 != 210000 || got.State != "overdue" {
		t.Fatalf("after delete: %d %s", got.NextDueKm.Int64, got.State)
	}
}

func TestTimeBasedAdvancesWithClock(t *testing.T) {
	f := newFixture(t, 100000)
	r, err := f.rem.Create(f.ctx, Input{
		VehicleID: f.vehicleID, Title: "Brake fluid", IntervalMonths: i64(24), StartDate: "2024-11-10",
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	// 2026-10-30 is within 30 days of 2026-11-10.
	if r.State != "due_soon" || r.LeadDays.Int64 != 30 {
		t.Fatalf("created: %s lead %d", r.State, r.LeadDays.Int64)
	}
	f.clk.Advance(11 * 24 * time.Hour)
	if err := f.rem.EvaluateAll(f.ctx); err != nil {
		t.Fatal(err)
	}
	if got := f.reload(t, r.ID); got.State != "overdue" || got.StateTrigger.String != "time" {
		t.Fatalf("after advance: %s/%s", got.State, got.StateTrigger.String)
	}
}

func TestPausedAndArchivedDoNotNotify(t *testing.T) {
	f := newFixture(t, 214500)
	r := f.oilReminder(t, "2026-08-01", 205500)
	f.transitions = nil // creating it at 214,500 km already entered due_soon
	if _, err := f.rem.SetPaused(f.ctx, r.ID, true); err != nil {
		t.Fatal(err)
	}
	f.odometer(t, 215600)
	if len(f.transitions) != 0 {
		t.Fatalf("paused reminder notified: %+v", f.transitions)
	}
	if got := f.reload(t, r.ID); got.State != "overdue" {
		t.Fatalf("paused reminder state not kept current: %s", got.State)
	}

	g := newFixture(t, 214500)
	r2 := g.oilReminder(t, "2026-08-01", 205500)
	g.transitions = nil
	if _, err := g.veh.SetArchived(g.ctx, g.vehicleID, true); err != nil {
		t.Fatal(err)
	}
	g.odometer(t, 215600)
	if len(g.transitions) != 0 || g.reload(t, r2.ID).LastNotifiedState != "overdue" {
		t.Fatalf("archived vehicle notified: %+v", g.transitions)
	}
}

func TestMarkDoneStandalone(t *testing.T) {
	f := newFixture(t, 100000)
	r, err := f.rem.Create(f.ctx, Input{VehicleID: f.vehicleID, Title: "Inspect CV boots", IntervalKm: i64(20000), StartDate: "2026-01-01", StartOdometerKm: i64(81000)}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if r.State != "due_soon" {
		t.Fatalf("state %s", r.State)
	}
	done, err := f.rem.MarkDone(f.ctx, r.ID, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if done.State != "upcoming" || done.NextDueKm.Int64 != 120000 || done.LastDoneDate.String != "2026-10-30" {
		t.Fatalf("done: %+v", done)
	}
	linked := f.oilReminder(t, "2026-08-01", 95000)
	if _, err := f.rem.MarkDone(f.ctx, linked.ID, "", nil); err == nil {
		t.Fatal("linked reminders must use the service log")
	}
}

func TestValidation(t *testing.T) {
	f := newFixture(t, 1000)
	cases := map[string]Input{
		"no interval":      {VehicleID: f.vehicleID, Title: "x"},
		"months and days":  {VehicleID: f.vehicleID, Title: "x", IntervalMonths: i64(6), IntervalDays: i64(10)},
		"lead too big km":  {VehicleID: f.vehicleID, Title: "x", IntervalKm: i64(10000), LeadKm: i64(12000)},
		"lead too big day": {VehicleID: f.vehicleID, Title: "x", IntervalMonths: i64(1), LeadDays: i64(28)},
		"no title":         {VehicleID: f.vehicleID, IntervalKm: i64(1000)},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := f.rem.Create(f.ctx, in, 0); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
