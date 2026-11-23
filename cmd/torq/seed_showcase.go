package main

import (
	"context"
	"fmt"
	"time"

	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/reminders"
	"github.com/app-torq/torq/internal/repairs"
	"github.com/app-torq/torq/internal/servicelog"
	"github.com/app-torq/torq/internal/vehicles"
)

// showcaseProject is an open (or completed) project for the demo board.
type showcaseProject struct {
	in       repairs.Input
	priority string // built-in key, "default", or "" for none
	parts    []repairs.PartInput
	// completedOn and completionKm complete the project when set.
	completedOn  string
	completionKm int64
}

// seedShowcase adds vehicles and projects that show the newer features: a
// JDM chassis number, per-vehicle odometer, displacement and power units,
// project priorities and tags. Its service costs are dated 2025 and its 2026
// projects are open, so the V7 totals for 2026 stay as documented.
func seedShowcase(ctx context.Context, clk *clock.Fixed, veh *vehicles.Service, svc *servicelog.Service, rep *repairs.Service, rem *reminders.Service, c220 int64) error {
	end := clk.At
	clk.At = time.Date(2025, 3, 1, 12, 0, 0, 0, time.Local)
	supra, err := veh.Create(ctx, vehicles.Profile{
		Make: "Toyota", Model: "Supra RZ", ChassisCode: "JZA80", ModelYear: i64(1997), EngineCode: "2JZ-GTE",
		VIN: "JZA80-0012345", LicensePlate: "HH-JZ 80", DisplacementCC: i64(2997), PowerW: i64(206000), FuelType: str("petrol"),
		OdometerUnit: "km", DisplacementUnit: str("cc"), PowerUnit: str("ps"),
	}, 141000, 0)
	if err != nil {
		return err
	}
	// A US car read in miles, shown in cubic inches and horsepower: 60,000 mi, 281 ci, 300 hp.
	mustang, err := veh.Create(ctx, vehicles.Profile{
		Make: "Ford", Model: "Mustang GT", ChassisCode: "S197", ModelYear: i64(2007), EngineCode: "4.6 3V",
		LicensePlate: "M-GT 4607", DisplacementCC: i64(4606), PowerW: i64(223710), FuelType: str("petrol"),
		OdometerUnit: "mi", DisplacementUnit: str("ci"), PowerUnit: str("hp"),
	}, 96561, 0)
	if err != nil {
		clk.At = end
		return err
	}
	// A tractor without an odometer, maintained by operating hours (#24).
	tractor, err := veh.Create(ctx, vehicles.Profile{
		Make: "Deutz", Model: "D 6206", ModelYear: i64(1976), EngineCode: "F6L 912", DisplacementCC: i64(5655), PowerW: i64(46000),
		FuelType: str("diesel"), NoOdometer: true,
		InitialMeter: &vehicles.MeterInput{Name: "Engine hours", HoursTenths: 12504, ReadingDate: "2025-03-01"},
	}, 0, 0)
	clk.At = end
	if err != nil {
		return err
	}
	tractorMeters, err := veh.Meters(ctx, []int64{tractor.ID})
	if err != nil {
		return err
	}
	tractorMeter := tractorMeters[tractor.ID][0].ID

	logs := []servicelog.Input{
		{VehicleID: supra.ID, ServiceDate: "2025-04-12", OdometerKm: 143200, Notes: "Oil change after winter storage.",
			Items: []servicelog.ItemInput{{CategoryID: 1, Brand: "Castrol Edge", Specification: "5W-40", QuantityMl: i64(5500), QuantityUnit: "L", OEMPartNumber: "90915-YZZJ1", CostCents: i64(8900)}}},
		{VehicleID: supra.ID, ServiceDate: "2025-09-20", OdometerKm: 148900,
			Items: []servicelog.ItemInput{
				{CategoryID: 1, Brand: "Castrol Edge", Specification: "5W-40", QuantityMl: i64(5500), QuantityUnit: "L", CostCents: i64(8900)},
				{CategoryID: 9, Brand: "NGK", Specification: "BKR7EIX", OEMPartNumber: "90919-01247", CostCents: i64(7800)},
			}},
		{VehicleID: mustang.ID, ServiceDate: "2025-05-03", OdometerKm: 98800, Notes: "Motorcraft filter FL-820-S.",
			Items: []servicelog.ItemInput{{CategoryID: 1, Brand: "Motorcraft", Specification: "5W-20", QuantityMl: i64(5394), QuantityUnit: "qt", CostCents: i64(7400)}}},
		{VehicleID: mustang.ID, ServiceDate: "2025-12-06", OdometerKm: 104600,
			Items: []servicelog.ItemInput{
				{CategoryID: 1, Brand: "Motorcraft", Specification: "5W-20", QuantityMl: i64(5394), QuantityUnit: "qt", CostCents: i64(7400)},
				{CategoryID: 6, Brand: "Motorcraft", AftermarketPartNumber: "FA-1883", CostCents: i64(2900)},
			}},
	}
	tractorHours := func(tenths int64) []vehicles.HoursReading {
		return []vehicles.HoursReading{{MeterID: tractorMeter, HoursTenths: tenths}}
	}
	logs = append(logs,
		servicelog.Input{VehicleID: tractor.ID, ServiceDate: "2025-04-26", NoOdometerKm: true, MeterReadings: tractorHours(12620), Notes: "Oil change before the hay season.",
			Items: []servicelog.ItemInput{
				{CategoryID: 1, Brand: "Fuchs Titan", Specification: "15W-40", QuantityMl: i64(14000), QuantityUnit: "L", CostCents: i64(7800)},
				{CategoryID: 8, OEMPartNumber: "01174421", CostCents: i64(2400)},
			}},
		servicelog.Input{VehicleID: tractor.ID, ServiceDate: "2025-12-06", NoOdometerKm: true, MeterReadings: tractorHours(14105),
			Items: []servicelog.ItemInput{
				{CategoryID: 1, Brand: "Fuchs Titan", Specification: "15W-40", QuantityMl: i64(14000), QuantityUnit: "L", CostCents: i64(7800)},
				{CategoryID: 6, OEMPartNumber: "01182654", CostCents: i64(3900)},
			}},
	)
	for _, in := range logs {
		if _, _, err := svc.Create(ctx, in, 0); err != nil {
			return err
		}
	}
	oil := int64(1)
	for _, r := range []reminders.Input{
		{VehicleID: supra.ID, ServiceCategoryID: &oil, IntervalKm: i64(10000), IntervalMonths: i64(12)},
		// 5,000 mi.
		{VehicleID: mustang.ID, ServiceCategoryID: &oil, IntervalKm: i64(8047), IntervalMonths: i64(12)},
		// Every 250 h or 12 months: last done at 1,410.5 h, due at 1,660.5 h.
		{VehicleID: tractor.ID, ServiceCategoryID: &oil, HourMeterID: &tractorMeter, IntervalHours: i64(250), IntervalMonths: i64(12)},
		{VehicleID: tractor.ID, Title: "Grease fittings", HourMeterID: &tractorMeter, IntervalHours: i64(50), LeadHours: i64(5)},
	} {
		if _, err := rem.Create(ctx, r, 0); err != nil {
			return err
		}
	}
	// This season's hours: oil due soon by hours (18 h left), greasing overdue.
	if _, err := veh.QuickUpdateHours(ctx, tractorMeter, vehicles.HoursUpdate{HoursTenths: 16420, ReadingDate: "2026-11-01"}); err != nil {
		return err
	}

	projects := []showcaseProject{
		{in: repairs.Input{VehicleID: supra.ID, Title: "Brake refresh before track day", Status: repairs.StatusInProgress, StartedOn: "2026-11-02",
			Symptom: "Insufficient braking power and a soft pedal after repeated hard stops.", RootCause: "Old fluid has boiled; pads glazed.",
			SpecialTools: "Pressure bleeder\nCaliper piston tool", Tags: []string{"brakes", "brake-fluid", "Brembo", "front axle", "track day"},
			TorqueSpecs: []repairs.TorqueSpec{{Label: "Front caliper bracket bolt", ValueNm: 108}, {Label: "Wheel nut", ValueNm: 103}}},
			priority: "urgent",
			parts: []repairs.PartInput{
				{Name: "Front brake pads", AftermarketBrand: "Brembo", AftermarketPartNumber: "P 83 027", Quantity: 1, UnitCostCents: i64(8900), Status: "ordered"},
				{Name: "Front brake discs", AftermarketBrand: "Brembo", AftermarketPartNumber: "09.5802.21", Quantity: 2, UnitCostCents: i64(7450), Status: "in_stock"},
				{Name: "Brake fluid DOT 5.1, 1 L", AftermarketBrand: "ATE", Quantity: 1, UnitCostCents: i64(1890), Status: "in_stock"},
			}},
		{in: repairs.Input{VehicleID: supra.ID, Title: "Sequential turbo vacuum lines", Status: repairs.StatusWaitingOnParts, StartedOn: "2026-10-25",
			Symptom: "Boost spike around 4,000 rpm during the turbo transition.", RootCause: "Cracked vacuum lines on the actuator side.",
			Tags: []string{"turbo", "vacuum lines", "engine"}},
			priority: "high",
			parts:    []repairs.PartInput{{Name: "Silicone vacuum hose kit", AftermarketBrand: "Samco", Quantity: 1, UnitCostCents: i64(12900), Status: "ordered"}}},
		{in: repairs.Input{VehicleID: supra.ID, Title: "Rear arch rust repair", Status: repairs.StatusPlanned,
			Symptom: "Bubbling paint on both rear wheel arches.", Tags: []string{"rust", "bodywork", "restoration"}},
			priority: "normal"},
		{in: repairs.Input{VehicleID: supra.ID, Title: "Clutch replacement", Status: repairs.StatusPlanned, StartedOn: "2025-06-07",
			Symptom: "Clutch slips in 3rd and 4th under full boost.", RootCause: "Worn clutch disc.", Tags: []string{"drivetrain", "clutch", "Exedy"}},
			priority:    "high",
			parts:       []repairs.PartInput{{Name: "Clutch kit", AftermarketBrand: "Exedy", AftermarketPartNumber: "TYK1501", Quantity: 1, UnitCostCents: i64(54900), Status: "installed"}},
			completedOn: "2025-06-14", completionKm: 145100},
		{in: repairs.Input{VehicleID: mustang.ID, Title: "Coil pack misfire, cylinder 2", Status: repairs.StatusInProgress, StartedOn: "2026-11-10",
			Symptom: "Check engine light and a stumble under load.", FaultCodes: []string{"P0302", "P0300"}, Tags: []string{"ignition", "engine", "Motorcraft"}},
			priority: "high",
			parts: []repairs.PartInput{
				{Name: "Ignition coil", OEMPartNumber: "3R2Z-12029-BA", AftermarketBrand: "Motorcraft", AftermarketPartNumber: "DG-511", Quantity: 1, UnitCostCents: i64(6900), Status: "in_stock"},
				{Name: "Spark plug", AftermarketBrand: "Motorcraft", AftermarketPartNumber: "SP-515", Quantity: 8, UnitCostCents: i64(1150), Status: "needed"},
			}},
		{in: repairs.Input{VehicleID: mustang.ID, Title: "LED headlight upgrade", Status: repairs.StatusPlanned, Tags: []string{"upgrade", "electrical", "cosmetic"}},
			priority: "low"},
		{in: repairs.Input{VehicleID: c220, Title: "Glow plug replacement", Status: repairs.StatusPlanned,
			Symptom: "Long cranking below 0 °C, glow plug light flashes.", FaultCodes: []string{"P0671", "P0674"},
			SpecialTools: "Glow plug extractor", Tags: []string{"diesel", "engine", "glow plugs"},
			TorqueSpecs: []repairs.TorqueSpec{{Label: "Glow plug", ValueNm: 12}}},
			priority: "default",
			parts:    []repairs.PartInput{{Name: "Glow plug", OEMPartNumber: "A0011597101", AftermarketBrand: "Bosch", Quantity: 4, UnitCostCents: i64(2490), Status: "needed"}}},
		{in: repairs.Input{VehicleID: tractor.ID, Title: "Hydraulic hose leak at the lift", Status: repairs.StatusPlanned,
			Symptom: "Oil drips under the three-point lift after work.", Tags: []string{"hydraulics", "restoration"}},
			priority: "normal",
			parts:    []repairs.PartInput{{Name: "Hydraulic hose 3/8\"", Quantity: 1, UnitCostCents: i64(4200), Status: "needed"}}},
		{in: repairs.Input{VehicleID: c220, Title: "Interior trim squeak", Status: repairs.StatusPlanned,
			Symptom: "Squeak from the passenger-side dashboard over bumps.", Tags: []string{"interior", "cosmetic"}}},
	}
	prios, err := rep.ListPriorities(ctx)
	if err != nil {
		return err
	}
	for _, p := range projects {
		switch p.priority {
		case "default":
			p.in.DefaultPriority = true
		case "":
		default:
			for _, pr := range prios {
				if pr.BuiltinKey.String == p.priority {
					p.in.PriorityID = &pr.ID
				}
			}
		}
		r, err := rep.Create(ctx, p.in, 0)
		if err != nil {
			return fmt.Errorf("%s: %w", p.in.Title, err)
		}
		for _, part := range p.parts {
			if _, err := rep.CreatePart(ctx, r.ID, part); err != nil {
				return err
			}
		}
		if p.completedOn != "" {
			if _, err := rep.SetStatus(ctx, r.ID, repairs.StatusCompleted, p.completedOn, &p.completionKm); err != nil {
				return err
			}
		}
	}
	fmt.Printf("seeded showcase: vehicles %d (JDM, cm³/PS), %d (mi, ci/hp) and %d (hours only), %d projects with priorities and tags\n", supra.ID, mustang.ID, tractor.ID, len(projects))
	return seedMotorcycle(ctx, clk, veh, svc, rem)
}
