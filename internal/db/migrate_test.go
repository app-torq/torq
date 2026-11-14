package db

import (
	"context"
	"database/sql"
	"io/fs"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

// TestVehicleCatalogMigrationKeepsVehicles covers migration 00007: existing
// vehicles keep every value and get NULL for the new profile fields (FR-026).
func TestVehicleCatalogMigrationKeepsVehicles(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, d.Writer, sub)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(ctx, 6); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Writer.ExecContext(ctx, `INSERT INTO vehicles
		(id, make, model, chassis_code, model_year, engine_code, vin, license_plate, current_odometer_km, created_at, updated_at)
		VALUES (1, 'Mercedes-Benz', 'C 220 CDI', 'S203', 2005, 'OM646', 'WDB2032081F123456', 'B-TQ 646', 215000,
		        '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, d, dir); err != nil {
		t.Fatal(err)
	}

	var make, model, chassis, engine, vin, plate string
	var year, km int64
	var hsn, tsn, fuel sql.NullString
	var cc, kw sql.NullInt64
	if err := d.Reader.QueryRowContext(ctx, `SELECT make, model, chassis_code, model_year, engine_code, vin, license_plate,
		current_odometer_km, hsn, tsn, displacement_cc, power_kw, fuel_type FROM vehicles WHERE id = 1`).
		Scan(&make, &model, &chassis, &year, &engine, &vin, &plate, &km, &hsn, &tsn, &cc, &kw, &fuel); err != nil {
		t.Fatal(err)
	}
	if make != "Mercedes-Benz" || model != "C 220 CDI" || chassis != "S203" || year != 2005 || engine != "OM646" ||
		vin != "WDB2032081F123456" || plate != "B-TQ 646" || km != 215000 {
		t.Fatalf("existing values changed: %s %s %s %d %s %s %s %d", make, model, chassis, year, engine, vin, plate, km)
	}
	if hsn.Valid || tsn.Valid || cc.Valid || kw.Valid || fuel.Valid {
		t.Fatalf("new fields must be NULL: %v %v %v %v %v", hsn, tsn, cc, kw, fuel)
	}

	var metaRows int
	if err := d.Reader.QueryRowContext(ctx, "SELECT count(*) FROM catalog_meta").Scan(&metaRows); err != nil {
		t.Fatal(err)
	}
	if metaRows != 1 {
		t.Fatalf("catalog_meta rows = %d", metaRows)
	}

	for _, set := range []string{
		"hsn = '07A0'", "hsn = '071'", "tsn = 'ab1'", "tsn = 'ABCD'", "power_kw = 0", "power_kw = 2001",
		"displacement_cc = 0", "displacement_cc = 20001", "fuel_type = 'kerosene'",
	} {
		if _, err := d.Writer.ExecContext(ctx, "UPDATE vehicles SET "+set+" WHERE id = 1"); err == nil {
			t.Errorf("CHECK accepted %s", set)
		}
	}
	if _, err := d.Writer.ExecContext(ctx, `UPDATE vehicles SET hsn = '0710', tsn = 'AB1', power_kw = 110,
		displacement_cc = 2148, fuel_type = 'diesel' WHERE id = 1`); err != nil {
		t.Fatalf("valid values rejected: %v", err)
	}
}

// TestVehicleRebuildMigrationKeepsData covers migration 00012, which rebuilds
// vehicles: rows and references survive, power_w is filled, foreign keys are
// enforced again afterwards and the looser VIN check applies.
func TestVehicleRebuildMigrationKeepsData(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, d.Writer, sub)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(ctx, 11); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO vehicles (id, make, model, vin, current_odometer_km, power_kw, created_at, updated_at)
		 VALUES (1, 'Mercedes-Benz', 'C 220 CDI', 'WDB2032081F123456', 215000, 110, 'x', 'x')`,
		`INSERT INTO vehicles (id, make, model, current_odometer_km, created_at, updated_at) VALUES (2, 'BMW', '330d', 1, 'x', 'x')`,
		`INSERT INTO service_logs (id, vehicle_id, service_date, odometer_km, created_at, updated_at)
		 VALUES (1, 1, '2026-10-01', 214000, 'x', 'x')`,
		`INSERT INTO reminders (vehicle_id, title, interval_km, start_date, start_odometer_km, state_changed_at, created_at, updated_at)
		 VALUES (1, 'Oil', 15000, '2026-10-01', 214000, 'x', 'x', 'x')`,
	} {
		if _, err := d.Writer.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(ctx, d, dir); err != nil {
		t.Fatal(err)
	}

	var vin, unit string
	var kw, w sql.NullInt64
	if err := d.Reader.QueryRowContext(ctx, "SELECT vin, odometer_unit, power_kw, power_w FROM vehicles WHERE id = 1").
		Scan(&vin, &unit, &kw, &w); err != nil {
		t.Fatal(err)
	}
	if vin != "WDB2032081F123456" || unit != "km" || kw.Int64 != 110 || w.Int64 != 110000 {
		t.Fatalf("vehicle 1: %s %s %v %v", vin, unit, kw, w)
	}
	if err := d.Reader.QueryRowContext(ctx, "SELECT power_w FROM vehicles WHERE id = 2").Scan(&w); err != nil || w.Valid {
		t.Fatalf("vehicle 2 power_w = %v (%v)", w, err)
	}
	var unknown int64
	if err := d.Reader.QueryRowContext(ctx, "SELECT last_done_unknown FROM reminders").Scan(&unknown); err != nil || unknown != 0 {
		t.Fatalf("last_done_unknown = %d (%v)", unknown, err)
	}

	var fk int
	if err := d.Writer.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		t.Fatalf("foreign_keys = %d (%v)", fk, err)
	}
	rows, err := d.Writer.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	if rows.Next() {
		t.Fatal("foreign key violations after migration")
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := d.Writer.ExecContext(ctx, "UPDATE vehicles SET vin = 'JZA80-0012345' WHERE id = 2"); err != nil {
		t.Fatalf("short VIN rejected: %v", err)
	}
	for _, set := range []string{"vin = ''", "vin = '" + strings.Repeat("A", 33) + "'", "odometer_unit = 'nm'", "power_w = 0"} {
		if _, err := d.Writer.ExecContext(ctx, "UPDATE vehicles SET "+set+" WHERE id = 2"); err == nil {
			t.Errorf("CHECK accepted %s", set)
		}
	}
	// References into the rebuilt table still cascade.
	if _, err := d.Writer.ExecContext(ctx, "DELETE FROM vehicles WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := d.Reader.QueryRowContext(ctx, "SELECT (SELECT count(*) FROM service_logs) + (SELECT count(*) FROM reminders)").Scan(&left); err != nil || left != 0 {
		t.Fatalf("cascade left %d rows (%v)", left, err)
	}
}
