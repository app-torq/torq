package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"time"

	"github.com/app-torq/torq/internal/auth"
	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/config"
	"github.com/app-torq/torq/internal/db"
	"github.com/app-torq/torq/internal/reminders"
	"github.com/app-torq/torq/internal/repairs"
	"github.com/app-torq/torq/internal/servicelog"
	"github.com/app-torq/torq/internal/vehicles"
)

// Demo credentials created by seed-demo when the instance has no users yet.
const (
	demoUser     = "demo"
	demoPassword = "password123" //nolint:gosec // documented demo credential for the TORQ_ENABLE_SEED dev command only
)

// seedDemo fills the database with sample data (development and validation
// only; guarded by TORQ_ENABLE_SEED=true).
//
// Without flags it creates the quickstart V7 dataset: one vehicle with 2026
// services totalling €420, completed repairs totalling €1,180 and readings
// from 205,000 to 225,000 km. With --vehicles/--years it generates bulk
// history for footprint checks (quickstart V9).
func seedDemo(args []string) error {
	if os.Getenv("TORQ_ENABLE_SEED") != "true" {
		return errors.New("seed-demo is disabled; set TORQ_ENABLE_SEED=true")
	}
	fs := flag.NewFlagSet("seed-demo", flag.ContinueOnError)
	nVehicles := fs.Int("vehicles", 0, "generate this many vehicles with bulk history")
	years := fs.Int("years", 5, "years of history per vehicle (with --vehicles)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(nil)
	if err != nil {
		return err
	}
	ctx := context.Background()
	d, err := db.Open(cfg.DataDir)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	if err := db.Migrate(ctx, d, cfg.DataDir); err != nil {
		return err
	}

	// Seeded history is backdated; a clock far enough ahead accepts every date.
	clk := &clock.Fixed{At: time.Date(2026, 12, 31, 12, 0, 0, 0, time.Local)}
	veh := &vehicles.Service{DB: d, Clock: clk}
	rem := &reminders.Service{DB: d, Clock: clock.System{}, Vehicles: veh}
	veh.AfterOdometerChange = rem.EvaluateVehicle
	svc := &servicelog.Service{DB: d, Clock: clk, Vehicles: veh}
	rep := &repairs.Service{DB: d, Clock: clk, Vehicles: veh}

	users := &auth.Users{DB: d, Clock: clock.System{}}
	if required, err := users.SetupRequired(ctx); err != nil {
		return err
	} else if required {
		if _, err := users.Setup(ctx, auth.NewUser{Username: demoUser, DisplayName: "Demo", Password: demoPassword, Locale: "en"}); err != nil {
			return err
		}
		fmt.Printf("created admin %q with password %q\n", demoUser, demoPassword)
	}

	if *nVehicles > 0 {
		return seedBulk(ctx, veh, svc, rep, rem, *nVehicles, *years)
	}
	return seedQuickstart(ctx, clk, veh, svc, rep)
}

func i64(v int64) *int64 { return &v }

func seedQuickstart(ctx context.Context, clk *clock.Fixed, veh *vehicles.Service, svc *servicelog.Service, rep *repairs.Service) error {
	year := int64(2006)
	// Date the initial reading before 2026 so the 2026 report spans exactly 205,000 → 225,000 km.
	end := clk.At
	clk.At = time.Date(2025, 11, 1, 12, 0, 0, 0, time.Local)
	defer func() { clk.At = end }()
	v, err := veh.Create(ctx, vehicles.Profile{Make: "Mercedes-Benz", Model: "C 220 CDI", ChassisCode: "S203", ModelYear: &year, EngineCode: "OM646", LicensePlate: "B-TQ 646"}, 204000, 0)
	if err != nil {
		return err
	}
	clk.At = end
	logs := []servicelog.Input{
		{VehicleID: v.ID, ServiceDate: "2026-01-10", OdometerKm: 205000, ExtraCostCents: 2000, ExtraCostLabel: "Shop supplies", Notes: "Oil change before winter.",
			Items: []servicelog.ItemInput{{CategoryID: 1, Brand: "Mobil 1", Specification: "MB 229.51", QuantityMl: i64(6500), QuantityUnit: "L", OEMPartNumber: "A6511800109", CostCents: i64(9000)}}},
		{VehicleID: v.ID, ServiceDate: "2026-06-01", OdometerKm: 215000,
			Items: []servicelog.ItemInput{{CategoryID: 6, CostCents: i64(2490)}, {CategoryID: 7, CostCents: i64(1990)}, {CategoryID: 3, Specification: "DOT 4", QuantityMl: i64(1000), QuantityUnit: "L", CostCents: i64(26520)}}},
	}
	for _, in := range logs {
		if _, _, err := svc.Create(ctx, in, 0); err != nil {
			return err
		}
	}
	r, err := rep.Create(ctx, repairs.Input{VehicleID: v.ID, Title: "Injector seal replacement", Symptom: "Rough idle on cold start, black smoke",
		RootCause: "Leaking copper seals on cylinders 1 and 3", FaultCodes: []string{"P0201", "P0300"}, SpecialTools: "E12 Torx\nInjector puller",
		TorqueSpecs: []repairs.TorqueSpec{{Label: "Injector clamp bolt", ValueNm: 7, Note: "+ 90°"}}}, 0)
	if err != nil {
		return err
	}
	for _, p := range []repairs.PartInput{
		{Name: "Injector copper seal", OEMPartNumber: "A6110170060", Quantity: 4, UnitCostCents: i64(2500), Status: "installed"},
		{Name: "Injector clamp bolt", OEMPartNumber: "A6119900301", Quantity: 4, UnitCostCents: i64(20000), Status: "installed"},
		{Name: "Return line", Quantity: 1, UnitCostCents: i64(28000), Status: "installed"},
	} {
		if _, err := rep.CreatePart(ctx, r.ID, p); err != nil {
			return err
		}
	}
	if _, err := rep.SetStatus(ctx, r.ID, repairs.StatusCompleted, "2026-11-20", i64(225000)); err != nil {
		return err
	}
	fmt.Printf("seeded vehicle %d: services €420, repairs €1,180, readings 205,000 → 225,000 km\n", v.ID)
	return nil
}

var demoModels = []struct{ make, model, chassis, engine string }{
	{"BMW", "330d", "E46", "M57"}, {"Volkswagen", "Golf GTI", "Mk5", "BWA"}, {"Mazda", "MX-5", "NB", "BP-4W"},
	{"Volvo", "V70", "P2", "B5244S"}, {"Mercedes-Benz", "E 320 CDI", "W211", "OM648"}, {"Toyota", "Land Cruiser", "J120", "1KD-FTV"},
}

func seedBulk(ctx context.Context, veh *vehicles.Service, svc *servicelog.Service, rep *repairs.Service, rem *reminders.Service, n, years int) error {
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // deterministic demo data, not security relevant
	start := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC).AddDate(-years, 0, 0)
	for i := 0; i < n; i++ {
		m := demoModels[i%len(demoModels)]
		km := int64(80000 + rng.IntN(120000))
		v, err := veh.Create(ctx, vehicles.Profile{Make: m.make, Model: m.model, ChassisCode: m.chassis, EngineCode: m.engine}, km, 0)
		if err != nil {
			return err
		}
		for day := start; day.Year() <= 2026; day = day.AddDate(0, 2+rng.IntN(3), rng.IntN(20)) {
			km += int64(2000 + rng.IntN(4000))
			items := []servicelog.ItemInput{{CategoryID: 1, Brand: "Castrol", Specification: "5W-30", QuantityMl: i64(5500), QuantityUnit: "L", CostCents: i64(int64(6000 + rng.IntN(4000)))}}
			if rng.IntN(3) == 0 {
				items = append(items, servicelog.ItemInput{CategoryID: int64(6 + rng.IntN(3)), CostCents: i64(int64(1500 + rng.IntN(3000)))})
			}
			if _, _, err := svc.Create(ctx, servicelog.Input{VehicleID: v.ID, ServiceDate: day.Format("2006-01-02"), OdometerKm: km, Items: items}, 0); err != nil {
				return err
			}
			if rng.IntN(5) == 0 {
				r, err := rep.Create(ctx, repairs.Input{VehicleID: v.ID, Title: fmt.Sprintf("Repair #%d", rng.IntN(1000)), FaultCodes: []string{fmt.Sprintf("P0%03d", rng.IntN(999))}}, 0)
				if err != nil {
					return err
				}
				if _, err := rep.CreatePart(ctx, r.ID, repairs.PartInput{Name: "Part", Quantity: 1 + int64(rng.IntN(3)), UnitCostCents: i64(int64(1000 + rng.IntN(20000))), Status: "installed"}); err != nil {
					return err
				}
				if _, err := rep.SetStatus(ctx, r.ID, repairs.StatusCompleted, day.Format("2006-01-02"), i64(km)); err != nil {
					return err
				}
			}
		}
		oil := int64(1)
		if _, err := rem.Create(ctx, reminders.Input{VehicleID: v.ID, ServiceCategoryID: &oil, IntervalKm: i64(15000), IntervalMonths: i64(12)}, 0); err != nil {
			return err
		}
	}
	fmt.Printf("seeded %d vehicles with %d years of history\n", n, years)
	return nil
}
