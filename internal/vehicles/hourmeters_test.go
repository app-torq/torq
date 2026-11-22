package vehicles

import (
	"database/sql"
	"math"
	"testing"
)

func TestHoursConversion(t *testing.T) {
	for _, tc := range []struct {
		h    float64
		want int64
		ok   bool
	}{
		{1250.4, 12504, true}, {1250.44, 12504, true}, {1250.46, 12505, true}, {0, 0, true},
		{999999.9, MaxHoursTenths, true}, {1000000, 0, false}, {-0.1, 0, false}, {math.NaN(), 0, false}, {math.Inf(1), 0, false},
	} {
		got, ok := HoursToTenths(tc.h)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("HoursToTenths(%v) = %d, %v; want %d, %v", tc.h, got, ok, tc.want, tc.ok)
		}
	}
	if got := TenthsToHours(12504); got != 1250.4 {
		t.Fatalf("TenthsToHours %v", got)
	}
}

func TestHoursOnlyVehicle(t *testing.T) {
	s, ctx := newService(t)
	if _, err := s.Create(ctx, Profile{Make: "Deutz", Model: "D 6206", NoOdometer: true}, 0, 0); conflictCode(t, err) != "vehicle.tracking_required" {
		t.Fatalf("no meter: %v", err)
	}
	v, err := s.Create(ctx, Profile{Make: "Deutz", Model: "D 6206", NoOdometer: true, InitialMeter: &MeterInput{Name: " Engine hours ", HoursTenths: 12504}}, 5000, 0)
	if err != nil {
		t.Fatal(err)
	}
	if v.HasOdometer != 0 || v.CurrentOdometerKm != 0 {
		t.Fatalf("vehicle %+v", v)
	}
	readings, err := s.Readings(ctx, v.ID, "", "")
	if err != nil || len(readings) != 0 {
		t.Fatalf("odometer readings %v (%v)", readings, err)
	}
	meters, err := s.Meters(ctx, []int64{v.ID})
	if err != nil || len(meters[v.ID]) != 1 {
		t.Fatalf("meters %v (%v)", meters, err)
	}
	m := meters[v.ID][0]
	if m.Name != "Engine hours" || m.CurrentHoursTenths != 12504 {
		t.Fatalf("meter %+v", m)
	}
	if _, err := s.CreateMeter(ctx, v.ID, MeterInput{Name: "PTO", HoursTenths: 1}); conflictCode(t, err) != "hour_meter.limit_reached" {
		t.Fatalf("second meter: %v", err)
	}
	if err := s.DeleteMeter(ctx, m.ID); conflictCode(t, err) != "hour_meter.required" {
		t.Fatalf("delete only meter: %v", err)
	}

	// Switching tracking: a car can turn its odometer off only with a meter.
	car := createS203(t, s, 214500)
	if _, err := s.Update(ctx, car.ID, Profile{Make: "Mercedes-Benz", Model: "C 220 CDI", NoOdometer: true}); conflictCode(t, err) != "vehicle.tracking_required" {
		t.Fatalf("car without meter: %v", err)
	}
	if _, err := s.CreateMeter(ctx, car.ID, MeterInput{Name: "Generator", HoursTenths: 3120}); err != nil {
		t.Fatal(err)
	}
	off, err := s.Update(ctx, car.ID, Profile{Make: "Mercedes-Benz", Model: "C 220 CDI", NoOdometer: true})
	if err != nil || off.HasOdometer != 0 || off.CurrentOdometerKm != 214500 {
		t.Fatalf("odometer off: %+v (%v)", off, err)
	}
}

func TestHoursQuickUpdateRules(t *testing.T) {
	s, ctx := newService(t)
	v := createS203(t, s, 214500)
	m, err := s.CreateMeter(ctx, v.ID, MeterInput{Name: "Generator", HoursTenths: 12620})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.QuickUpdateHours(ctx, m.ID, HoursUpdate{HoursTenths: 12700}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.QuickUpdateHours(ctx, m.ID, HoursUpdate{HoursTenths: 11260}); conflictCode(t, err) != "hour_meter.decrease_requires_confirmation" {
		t.Fatalf("decrease: %v", err)
	}
	if _, err := s.QuickUpdateHours(ctx, m.ID, HoursUpdate{HoursTenths: 11260, ConfirmDecrease: true}); err != nil {
		t.Fatalf("confirmed decrease: %v", err)
	}
	// More than double, and more than 5,000 h above the current value.
	for _, tenths := range []int64{22600, 63000} {
		if _, err := s.QuickUpdateHours(ctx, m.ID, HoursUpdate{HoursTenths: tenths}); conflictCode(t, err) != "hour_meter.large_jump_requires_confirmation" {
			t.Fatalf("jump to %d: %v", tenths, err)
		}
	}
	if _, err := s.QuickUpdateHours(ctx, m.ID, HoursUpdate{HoursTenths: 12000, ReadingDate: "2026-10-31"}); err == nil {
		t.Fatal("future date accepted")
	}
	readings, err := s.HoursReadings(ctx, m.ID, "", "")
	// Same-day readings are ordered by value, like odometer readings.
	if err != nil || len(readings) != 3 || readings[0].HoursTenths != 12700 || readings[2].Source != SourceQuickUpdate {
		t.Fatalf("readings %+v (%v)", readings, err)
	}

	// A reading from an entry raises the value but never lowers it.
	err = s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		if err := s.RecordHoursReading(ctx, tx, "2026-10-30", SourceServiceLog, HoursReading{MeterID: m.ID, HoursTenths: 11000}); err != nil {
			return err
		}
		return s.RecordHoursReading(ctx, tx, "2026-10-30", SourceServiceLog, HoursReading{MeterID: m.ID, HoursTenths: 11500})
	})
	if err != nil {
		t.Fatal(err)
	}
	meters, _ := s.Meters(ctx, []int64{v.ID})
	if got := meters[v.ID][0].CurrentHoursTenths; got != 11500 {
		t.Fatalf("current after entries %d", got)
	}

	if _, err := s.RenameMeter(ctx, m.ID, "  "); err == nil {
		t.Fatal("empty name accepted")
	}
	if err := s.DeleteMeter(ctx, m.ID); err != nil {
		t.Fatalf("delete unused meter: %v", err)
	}
}
