package main

import (
	"context"
	"fmt"
	"time"

	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/reminders"
	"github.com/app-torq/torq/internal/servicelog"
	"github.com/app-torq/torq/internal/vehicles"
)

// seedMotorcycle adds a motorcycle with its spec sheet, two services with
// motorcycle categories and a chain reminder (#28). Values are illustrative.
// Services are dated 2025, so the V7 totals for 2026 stay as documented.
func seedMotorcycle(ctx context.Context, clk *clock.Fixed, veh *vehicles.Service, svc *servicelog.Service, rem *reminders.Service) error {
	end := clk.At
	clk.At = time.Date(2025, 3, 1, 12, 0, 0, 0, time.Local)
	bike, err := veh.Create(ctx, vehicles.Profile{
		VehicleType: vehicles.TypeMotorcycle, Make: "Yamaha", Model: "MT-07", ModelYear: i64(2018), DisplacementCC: i64(689),
		PowerW: i64(54000), FuelType: str("petrol"), LicensePlate: "HH-MT 07",
	}, 21300, 0)
	clk.At = end
	if err != nil {
		return err
	}
	f64 := func(v float64) *float64 { return &v }
	if _, err := veh.PutSpecs(ctx, bike.ID, vehicles.Specs{
		EngineOilViscosity: "10W-40", EngineOilSpec: "JASO MA2", EngineOilCapacityMl: i64(3000),
		BrakeFluidSpec: "DOT 4", CoolantSpec: "Yamalube Coolant", TirePressureFrontBar: f64(2.25), TirePressureRearBar: f64(2.5),
		Motorcycle: &vehicles.MotorcycleSpecs{
			FinalDrive: str("chain"), SprocketFrontTeeth: i64(16), SprocketRearTeeth: i64(43), ChainSize: "525 X-ring", ChainLinks: i64(108),
			ForkOilSpec: "SAE 10W", ForkOilCapacityMl: i64(495),
			ValveClearanceIntakeMinMm: f64(0.11), ValveClearanceIntakeMaxMm: f64(0.20),
			ValveClearanceExhaustMinMm: f64(0.24), ValveClearanceExhaustMaxMm: f64(0.30),
		},
	}); err != nil {
		return err
	}

	ids := map[string]int64{}
	cats, err := svc.ListCategories(ctx, false, vehicles.TypeMotorcycle)
	if err != nil {
		return err
	}
	for _, c := range cats {
		if c.BuiltinKey.Valid {
			ids[c.BuiltinKey.String] = c.ID
		}
	}
	for _, in := range []servicelog.Input{
		{VehicleID: bike.ID, ServiceDate: "2025-04-05", OdometerKm: 21800, Notes: "Season start.",
			Items: []servicelog.ItemInput{
				{CategoryID: ids["engine_oil"], Brand: "Motul 7100", Specification: "10W-40", QuantityMl: i64(3000), QuantityUnit: "L", CostCents: i64(4900)},
				{CategoryID: ids["chain_maintenance"], Brand: "Motul C2+", CostCents: i64(1400)},
			}},
		{VehicleID: bike.ID, ServiceDate: "2025-09-13", OdometerKm: 26900,
			Items: []servicelog.ItemInput{
				{CategoryID: ids["valve_clearance"], Specification: "Within limits, no shims changed", CostCents: i64(0)},
				{CategoryID: ids["chain_maintenance"], Brand: "Motul C2+", CostCents: i64(0)},
			}},
	} {
		if _, _, err := svc.Create(ctx, in, 0); err != nil {
			return err
		}
	}
	chain := ids["chain_maintenance"]
	if _, err := rem.Create(ctx, reminders.Input{VehicleID: bike.ID, ServiceCategoryID: &chain, IntervalKm: i64(1000)}, 0); err != nil {
		return err
	}
	fmt.Printf("seeded motorcycle: vehicle %d\n", bike.ID)
	return nil
}
