package db

import (
	"context"
	"database/sql"
	"io/fs"
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
