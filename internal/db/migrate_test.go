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
	for _, set := range []string{"vin = ''", "vin = '" + strings.Repeat("A", 33) + "'", "odometer_unit = 'nm'", "power_w = 0", "displacement_unit = 'm3'", "power_unit = 'bhp'"} {
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

// TestUnitPreferencesMigrationKeepsData covers migration 00013, which
// rebuilds users and service_log_items: preferences, references and items
// survive, new preferences start metric and the new choices are accepted.
func TestUnitPreferencesMigrationKeepsData(t *testing.T) {
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
	if _, err := p.UpTo(ctx, 12); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO users (id, username, display_name, role, locale, date_format, number_format, currency_code, currency_symbol, power_unit, created_at, updated_at)
		 VALUES (1, 'marek', 'Marek', 'admin', 'de', 'DD.MM.YYYY', 'comma', 'CHF', 'Fr.', 'ps', 'x', 'x')`,
		`INSERT INTO sessions (user_id, token_hash, created_at, last_seen_at, expires_at) VALUES (1, x'01', 'x', 'x', 'x')`,
		`INSERT INTO vehicles (id, make, model, current_odometer_km, created_by, created_at, updated_at) VALUES (1, 'BMW', '330d', 1, 1, 'x', 'x')`,
		`INSERT INTO service_logs (id, vehicle_id, service_date, odometer_km, created_by, created_at, updated_at) VALUES (1, 1, '2026-10-01', 1, 1, 'x', 'x')`,
		`INSERT INTO service_log_items (service_log_id, category_id, quantity_ml, quantity_unit, cost_type, sort_order) VALUES (1, 1, 6500, 'L', 'fluids', 1)`,
	} {
		if _, err := d.Writer.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(ctx, d, dir); err != nil {
		t.Fatal(err)
	}

	var locale, dateFmt, numFmt, currency, power, timeFmt, dist, vol, torque, pressure, disp string
	var symbol sql.NullString
	if err := d.Reader.QueryRowContext(ctx, `SELECT locale, date_format, number_format, currency_code, currency_symbol, power_unit,
		time_format, distance_unit, volume_unit, torque_unit, pressure_unit, displacement_unit FROM users WHERE id = 1`).
		Scan(&locale, &dateFmt, &numFmt, &currency, &symbol, &power, &timeFmt, &dist, &vol, &torque, &pressure, &disp); err != nil {
		t.Fatal(err)
	}
	if locale != "de" || dateFmt != "DD.MM.YYYY" || numFmt != "comma" || currency != "CHF" || symbol.String != "Fr." || power != "ps" {
		t.Fatalf("preferences changed: %s %s %s %s %v %s", locale, dateFmt, numFmt, currency, symbol, power)
	}
	if timeFmt != "24h" || dist != "km" || vol != "l" || torque != "nm" || pressure != "bar" || disp != "cc" {
		t.Fatalf("new preferences not metric: %s %s %s %s %s %s", timeFmt, dist, vol, torque, pressure, disp)
	}
	var ml int64
	var unit string
	if err := d.Reader.QueryRowContext(ctx, "SELECT quantity_ml, quantity_unit FROM service_log_items").Scan(&ml, &unit); err != nil || ml != 6500 || unit != "L" {
		t.Fatalf("item %d %s (%v)", ml, unit, err)
	}
	var defaults string
	if err := d.Reader.QueryRowContext(ctx, "SELECT default_distance_unit || default_date_format || default_currency_code FROM instance_settings").Scan(&defaults); err != nil || defaults != "kmYYYY-MM-DDEUR" {
		t.Fatalf("defaults %q (%v)", defaults, err)
	}

	var fk int
	if err := d.Writer.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		t.Fatalf("foreign_keys = %d (%v)", fk, err)
	}
	for _, stmt := range []string{
		"UPDATE users SET date_format = 'MM/DD/YYYY', power_unit = 'hp', time_format = '12h', distance_unit = 'mi', volume_unit = 'qt', torque_unit = 'lbft', pressure_unit = 'kpa', displacement_unit = 'ci'",
		"UPDATE service_log_items SET quantity_ml = 4732, quantity_unit = 'qt'",
		"UPDATE instance_settings SET default_distance_unit = 'mi', default_volume_unit = 'qt'",
	} {
		if _, err := d.Writer.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	for _, stmt := range []string{
		"UPDATE users SET distance_unit = 'nmi'", "UPDATE users SET date_format = 'DD/MM/YYYY'",
		"UPDATE service_log_items SET quantity_unit = 'pt'", "UPDATE instance_settings SET default_pressure_unit = 'atm'",
	} {
		if _, err := d.Writer.ExecContext(ctx, stmt); err == nil {
			t.Errorf("CHECK accepted %s", stmt)
		}
	}
	// References into the rebuilt users table still apply.
	if _, err := d.Writer.ExecContext(ctx, "DELETE FROM users WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	var sessions int
	var createdBy sql.NullInt64
	if err := d.Reader.QueryRowContext(ctx, "SELECT (SELECT count(*) FROM sessions), (SELECT created_by FROM vehicles WHERE id = 1)").Scan(&sessions, &createdBy); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 || createdBy.Valid {
		t.Fatalf("after user delete: %d sessions, created_by %v", sessions, createdBy)
	}
}

// TestRepairPrioritiesMigration covers migration 00014: the built-in
// priorities exist, existing projects and users keep working without a
// priority, and deleting a priority unsets it on its projects.
func TestRepairPrioritiesMigration(t *testing.T) {
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
	if _, err := p.UpTo(ctx, 13); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO users (id, username, display_name, role, created_at, updated_at) VALUES (1, 'marek', 'Marek', 'admin', 'x', 'x')`,
		`INSERT INTO vehicles (id, make, model, current_odometer_km, created_at, updated_at) VALUES (1, 'BMW', '330d', 1, 'x', 'x')`,
		`INSERT INTO repair_logs (id, vehicle_id, title, created_at, updated_at) VALUES (1, 1, 'Rust', 'x', 'x')`,
	} {
		if _, err := d.Writer.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(ctx, d, dir); err != nil {
		t.Fatal(err)
	}

	var keys string
	if err := d.Reader.QueryRowContext(ctx, `SELECT group_concat(builtin_key || ':' || color || ':' || is_default, ',')
		FROM (SELECT * FROM repair_priorities ORDER BY sort_order)`).Scan(&keys); err != nil {
		t.Fatal(err)
	}
	if keys != "urgent:red:0,high:amber:0,normal:sky:1,low:zinc:0" {
		t.Fatalf("built-ins %q", keys)
	}
	var prio sql.NullInt64
	var display string
	if err := d.Reader.QueryRowContext(ctx, "SELECT (SELECT priority_id FROM repair_logs WHERE id = 1), (SELECT priority_display FROM users WHERE id = 1)").Scan(&prio, &display); err != nil {
		t.Fatal(err)
	}
	if prio.Valid || display != "both" {
		t.Fatalf("existing rows: priority %v, display %q", prio, display)
	}
	for _, stmt := range []string{
		"UPDATE repair_priorities SET is_default = 1 WHERE builtin_key = 'low'",
		"INSERT INTO repair_priorities (custom_name, color, sort_order, created_at, updated_at) VALUES ('URGENT', 'red', 9, 'x', 'x'), ('urgent', 'red', 10, 'x', 'x')",
		"UPDATE users SET priority_display = 'icon'",
	} {
		if _, err := d.Writer.ExecContext(ctx, stmt); err == nil {
			t.Errorf("accepted %s", stmt)
		}
	}
	if _, err := d.Writer.ExecContext(ctx, "UPDATE repair_logs SET priority_id = (SELECT id FROM repair_priorities WHERE builtin_key = 'high')"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Writer.ExecContext(ctx, "DELETE FROM repair_priorities WHERE builtin_key = 'high'"); err != nil {
		t.Fatal(err)
	}
	if err := d.Reader.QueryRowContext(ctx, "SELECT priority_id FROM repair_logs WHERE id = 1").Scan(&prio); err != nil || prio.Valid {
		t.Fatalf("priority after delete %v (%v)", prio, err)
	}
}

// TestHourMetersMigration covers migration 00017: existing rows keep their
// values and get the odometer flags set, reminders survive their rebuild with
// their indexes and webhook references, and the new CHECKs apply.
func TestHourMetersMigration(t *testing.T) {
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
	if _, err := p.UpTo(ctx, 16); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO vehicles (id, make, model, current_odometer_km, created_at, updated_at) VALUES (1, 'Deutz', 'D 6206', 4000, 'x', 'x')`,
		`INSERT INTO service_logs (id, vehicle_id, service_date, odometer_km, created_at, updated_at) VALUES (1, 1, '2026-10-01', 3900, 'x', 'x')`,
		`INSERT INTO repair_logs (id, vehicle_id, title, status, completed_on, completion_odometer_km, created_at, updated_at)
		 VALUES (1, 1, 'Clutch', 'completed', '2026-10-02', 3950, 'x', 'x')`,
		`INSERT INTO reminders (id, vehicle_id, service_category_id, interval_km, interval_months, lead_km, start_date, start_odometer_km,
		                       next_due_km, next_due_date, state, state_trigger, state_changed_at, last_done_unknown, created_at, updated_at)
		 VALUES (1, 1, 1, 15000, 12, 1000, '2026-10-01', 3900, 18900, '2027-10-01', 'due_soon', 'km', 'x', 1, 'x', 'x')`,
		`INSERT INTO webhooks (id, name, url, preset, created_at, updated_at) VALUES (1, 'n', 'https://example.org', 'generic_json', 'x', 'x')`,
		`INSERT INTO webhook_deliveries (id, webhook_id, event, reminder_id, request_body, request_headers_json, status, created_at)
		 VALUES (1, 1, 'reminder_due_soon', 1, x'7b7d', '{}', 'succeeded', 'x')`,
	} {
		if _, err := d.Writer.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := Migrate(ctx, d, dir); err != nil {
		t.Fatal(err)
	}

	var hasOdo, logRec, repRec, unknown int64
	var intervalKm, leadKm, nextKm int64
	var state, trigger string
	if err := d.Reader.QueryRowContext(ctx, `SELECT (SELECT has_odometer FROM vehicles WHERE id = 1),
		(SELECT odometer_recorded FROM service_logs WHERE id = 1), (SELECT completion_odometer_recorded FROM repair_logs WHERE id = 1),
		interval_km, lead_km, next_due_km, state, state_trigger, last_done_unknown FROM reminders WHERE id = 1`).
		Scan(&hasOdo, &logRec, &repRec, &intervalKm, &leadKm, &nextKm, &state, &trigger, &unknown); err != nil {
		t.Fatal(err)
	}
	if hasOdo != 1 || logRec != 1 || repRec != 1 || intervalKm != 15000 || leadKm != 1000 || nextKm != 18900 || state != "due_soon" || trigger != "km" || unknown != 1 {
		t.Fatalf("after migration: %d %d %d %d %d %d %s %s %d", hasOdo, logRec, repRec, intervalKm, leadKm, nextKm, state, trigger, unknown)
	}
	var indexes string
	if err := d.Reader.QueryRowContext(ctx, "SELECT group_concat(name, ',') FROM (SELECT name FROM sqlite_master WHERE type = 'index' AND tbl_name = 'reminders' ORDER BY name)").Scan(&indexes); err != nil {
		t.Fatal(err)
	}
	if indexes != "reminders_category,reminders_hour_meter,reminders_vehicle_state" {
		t.Fatalf("indexes %q", indexes)
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

	for _, stmt := range []string{
		`INSERT INTO hour_meters (vehicle_id, name, current_hours_tenths, created_at, updated_at) VALUES (1, 'Engine hours', 12504, 'x', 'x')`,
		`INSERT INTO hour_meter_readings (hour_meter_id, reading_date, hours_tenths, source, created_at) VALUES (1, '2026-10-03', 12504, 'initial', 'x')`,
		`UPDATE reminders SET hour_meter_id = 1, interval_hours = 250, lead_hours = 25, state_trigger = 'hours' WHERE id = 1`,
	} {
		if _, err := d.Writer.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	for _, stmt := range []string{
		"UPDATE vehicles SET has_odometer = 2",
		"INSERT INTO hour_meters (vehicle_id, name, current_hours_tenths, created_at, updated_at) VALUES (1, 'Second', 1, 'x', 'x')",
		"INSERT INTO hour_meters (vehicle_id, name, current_hours_tenths, created_at, updated_at) VALUES (1, '', 1, 'x', 'x')",
		"INSERT INTO hour_meter_readings (hour_meter_id, reading_date, hours_tenths, source, created_at) VALUES (1, '2026-10-03', -1, 'initial', 'x')",
		"UPDATE reminders SET interval_km = NULL, interval_months = NULL, interval_hours = NULL, lead_hours = NULL",
		"UPDATE reminders SET state_trigger = 'meters'",
		"UPDATE reminders SET hour_meter_id = NULL",
	} {
		if _, err := d.Writer.ExecContext(ctx, stmt); err == nil {
			t.Errorf("CHECK accepted %s", stmt)
		}
	}
	// The webhook delivery still points at the rebuilt reminders table.
	if _, err := d.Writer.ExecContext(ctx, "DELETE FROM reminders WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	var reminderID sql.NullInt64
	if err := d.Reader.QueryRowContext(ctx, "SELECT reminder_id FROM webhook_deliveries WHERE id = 1").Scan(&reminderID); err != nil || reminderID.Valid {
		t.Fatalf("delivery reminder_id after delete %v (%v)", reminderID, err)
	}
}

// TestMotorcyclesMigration covers migration 00019: existing vehicles and
// catalog rows become cars, spec rows get empty motorcycle fields, the
// built-in categories gain their vehicle types and the CHECKs apply.
func TestMotorcyclesMigration(t *testing.T) {
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
	if _, err := p.UpTo(ctx, 17); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO vehicles (id, make, model, current_odometer_km, created_at, updated_at) VALUES (1, 'BMW', '330d', 4000, 'x', 'x')`,
		`INSERT INTO vehicle_specs (vehicle_id, engine_oil_spec, updated_at) VALUES (1, 'LL-04', 'x')`,
	} {
		if _, err := d.Writer.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := Migrate(ctx, d, dir); err != nil {
		t.Fatal(err)
	}

	var vehicleType, chainSize, oilSpec string
	var finalDrive sql.NullString
	var links sql.NullInt64
	if err := d.Reader.QueryRowContext(ctx, `SELECT vehicle_type, final_drive, chain_size, chain_links, engine_oil_spec
		FROM vehicles JOIN vehicle_specs ON vehicle_specs.vehicle_id = vehicles.id WHERE vehicles.id = 1`).
		Scan(&vehicleType, &finalDrive, &chainSize, &links, &oilSpec); err != nil {
		t.Fatal(err)
	}
	if vehicleType != "car" || finalDrive.Valid || chainSize != "" || links.Valid || oilSpec != "LL-04" {
		t.Fatalf("after migration: %s %v %q %v %s", vehicleType, finalDrive, chainSize, links, oilSpec)
	}

	types := map[string]string{}
	rows, err := d.Reader.QueryContext(ctx, `SELECT builtin_key, COALESCE(vehicle_type, '') || '|' || COALESCE(default_interval_km, '') || '|' ||
		COALESCE(default_interval_months, '') || '|' || is_fluid FROM service_categories WHERE builtin_key IS NOT NULL`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			t.Fatal(err)
		}
		types[k] = v
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"cabin_filter": "car|15000|12|0", "differential_fluid": "car|60000||1", "engine_oil": "|15000|12|1", "timing_belt_chain": "|120000|72|0",
		"chain_maintenance": "motorcycle|1000||0", "chain_sprockets": "motorcycle|||0", "valve_clearance": "motorcycle|24000||0",
		"fork_oil": "motorcycle||24|1", "final_drive_oil": "motorcycle||24|1", "drive_belt": "motorcycle|||0",
	}
	for k, v := range want {
		if types[k] != v {
			t.Errorf("%s = %q, want %q", k, types[k], v)
		}
	}
	if len(types) != 17 {
		t.Errorf("%d built-ins, want 17", len(types))
	}

	for _, stmt := range []string{
		"UPDATE vehicles SET vehicle_type = 'truck'",
		"UPDATE service_categories SET vehicle_type = 'truck'",
		"INSERT INTO catalog_vehicles (make, make_key, model, model_key, generation, generation_key, engine, engine_key, vehicle_type) VALUES ('a', 'a', 'b', 'b', '', '', 'c', 'c', 'truck')",
		"UPDATE vehicle_specs SET final_drive = 'cardan'",
		"UPDATE vehicle_specs SET chain_links = 10",
		"UPDATE vehicle_specs SET valve_clearance_intake_min_mm = 0",
	} {
		if _, err := d.Writer.ExecContext(ctx, stmt); err == nil {
			t.Errorf("CHECK accepted %s", stmt)
		}
	}
	// The catalog row above fails only because of its type.
	if _, err := d.Writer.ExecContext(ctx, "INSERT INTO catalog_vehicles (make, make_key, model, model_key, generation, generation_key, engine, engine_key, vehicle_type) VALUES ('a', 'a', 'b', 'b', '', '', 'c', 'c', 'motorcycle')"); err != nil {
		t.Fatal(err)
	}
}

// TestVehicleSharesMigration covers migration 00020: no vehicle is shared
// after the upgrade, links follow their vehicle and the constraints hold.
func TestVehicleSharesMigration(t *testing.T) {
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
	if _, err := p.UpTo(ctx, 19); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Writer.ExecContext(ctx, `INSERT INTO vehicles (id, make, model, current_odometer_km, created_at, updated_at) VALUES (1, 'BMW', '330d', 4000, 'x', 'x')`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, d, dir); err != nil {
		t.Fatal(err)
	}

	var shares, enabled int
	if err := d.Reader.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM vehicle_shares), public_links_enabled FROM instance_settings`).Scan(&shares, &enabled); err != nil {
		t.Fatal(err)
	}
	if shares != 0 || enabled != 1 {
		t.Fatalf("after migration: %d shares, enabled %d", shares, enabled)
	}

	insert := `INSERT INTO vehicle_shares (vehicle_id, token, token_sha256, created_at) VALUES (?, ?, ?, 'x')`
	if _, err := d.Writer.ExecContext(ctx, insert, 1, "a", []byte{1}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Writer.ExecContext(ctx, `INSERT INTO vehicles (id, make, model, current_odometer_km, created_at, updated_at) VALUES (2, 'VW', 'Golf', 1, 'x', 'x')`); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		stmt string
		args []any
	}{
		{insert, []any{1, "b", []byte{2}}}, // second link for one vehicle
		{insert, []any{2, "c", []byte{1}}}, // duplicate hash
		{"UPDATE vehicle_shares SET include_vin = 2", nil},
		{"UPDATE instance_settings SET public_links_enabled = 2", nil},
	} {
		if _, err := d.Writer.ExecContext(ctx, c.stmt, c.args...); err == nil {
			t.Errorf("accepted %s %v", c.stmt, c.args)
		}
	}
	if _, err := d.Writer.ExecContext(ctx, "DELETE FROM vehicles WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	if err := d.Reader.QueryRowContext(ctx, "SELECT count(*) FROM vehicle_shares").Scan(&shares); err != nil || shares != 0 {
		t.Fatalf("link survived its vehicle: %d %v", shares, err)
	}
}

// TestRolesMigration covers migration 00021: administrators and members keep
// what they could do, and the new tables enforce their constraints.
func TestRolesMigration(t *testing.T) {
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
	if _, err := p.UpTo(ctx, 20); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO users (id, username, display_name, role, created_at, updated_at) VALUES (1, 'admin', 'Admin', 'admin', 'x', 'x')`,
		`INSERT INTO users (id, username, display_name, role, created_at, updated_at) VALUES (2, 'anna', 'Anna', 'member', 'x', 'x')`,
		`INSERT INTO users (id, username, display_name, role, created_at, updated_at) VALUES (3, 'ben', 'Ben', 'member', 'x', 'x')`,
		`INSERT INTO vehicles (id, make, model, current_odometer_km, created_at, updated_at) VALUES (1, 'BMW', '330d', 4000, 'x', 'x')`,
	} {
		if _, err := d.Writer.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	if err := Migrate(ctx, d, dir); err != nil {
		t.Fatal(err)
	}

	roleOf := func(user int) string {
		var key string
		if err := d.Reader.QueryRowContext(ctx, `SELECT roles.builtin_key FROM users JOIN roles ON roles.id = users.role_id WHERE users.id = ?`, user).Scan(&key); err != nil {
			t.Fatal(err)
		}
		return key
	}
	if roleOf(1) != "admin" || roleOf(2) != "member" || roleOf(3) != "member" {
		t.Fatalf("roles after migration: %s %s %s", roleOf(1), roleOf(2), roleOf(3))
	}
	var member, viewer, admin, access int
	if err := d.Reader.QueryRowContext(ctx, `SELECT
		(SELECT count(*) FROM role_permissions JOIN roles ON roles.id = role_id WHERE builtin_key = 'member'),
		(SELECT count(*) FROM role_permissions JOIN roles ON roles.id = role_id WHERE builtin_key = 'viewer'),
		(SELECT count(*) FROM role_permissions JOIN roles ON roles.id = role_id WHERE builtin_key = 'admin'),
		(SELECT count(*) FROM vehicle_access)`).Scan(&member, &viewer, &admin, &access); err != nil {
		t.Fatal(err)
	}
	if member != 13 || viewer != 0 || admin != 0 || access != 0 {
		t.Fatalf("permissions: member %d, viewer %d, admin %d; access rows %d", member, viewer, admin, access)
	}

	if _, err := d.Writer.ExecContext(ctx, `INSERT INTO roles (custom_name, created_at, updated_at) VALUES ('Customer', 'x', 'x')`); err != nil {
		t.Fatal(err)
	}
	insertAccess := `INSERT INTO vehicle_access (user_id, vehicle_id, level, created_at) VALUES (?, ?, ?, 'x')`
	if _, err := d.Writer.ExecContext(ctx, insertAccess, 2, 1, "edit"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Writer.ExecContext(ctx, insertAccess, 3, 1, "view"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		stmt string
		args []any
	}{
		{`INSERT INTO roles (custom_name, created_at, updated_at) VALUES ('customer', 'x', 'x')`, nil}, // duplicate name
		{`INSERT INTO roles (created_at, updated_at) VALUES ('x', 'x')`, nil},                          // no name
		{`INSERT INTO roles (builtin_key, created_at, updated_at) VALUES ('owner', 'x', 'x')`, nil},    // unknown built-in
		{`UPDATE roles SET all_vehicles = 2`, nil},
		{insertAccess, []any{2, 1, "edit"}}, // duplicate
		{"UPDATE vehicle_access SET level = 'manage'", nil},
	} {
		if _, err := d.Writer.ExecContext(ctx, c.stmt, c.args...); err == nil {
			t.Errorf("accepted %s %v", c.stmt, c.args)
		}
	}
	if _, err := d.Writer.ExecContext(ctx, "DELETE FROM users WHERE id = 3"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Writer.ExecContext(ctx, "DELETE FROM vehicles WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	if err := d.Reader.QueryRowContext(ctx, "SELECT count(*) FROM vehicle_access").Scan(&access); err != nil || access != 0 {
		t.Fatalf("access rows survived their user or vehicle: %d %v", access, err)
	}
}

// TestTwoFactorMigration covers migration 00022: nobody needs a second factor
// after the update, and the credentials go with their user.
func TestTwoFactorMigration(t *testing.T) {
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
	if _, err := p.UpTo(ctx, 21); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Writer.ExecContext(ctx, `INSERT INTO users (id, username, display_name, role, role_id, created_at, updated_at)
		VALUES (1, 'anna', 'Anna', 'member', (SELECT id FROM roles WHERE builtin_key = 'member'), 'x', 'x')`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, d, dir); err != nil {
		t.Fatal(err)
	}

	var required, roles int
	if err := d.Reader.QueryRowContext(ctx, `SELECT (SELECT sum(mfa_required) FROM users), (SELECT sum(require_mfa) FROM roles)`).Scan(&required, &roles); err != nil {
		t.Fatal(err)
	}
	if required != 0 || roles != 0 {
		t.Fatalf("requirements after migration: users %d, roles %d", required, roles)
	}

	for _, stmt := range []string{
		`INSERT INTO user_totp (user_id, secret_enc, created_at) VALUES (1, x'00', 'x')`,
		`INSERT INTO user_recovery_codes (user_id, code_hash, created_at) VALUES (1, x'01', 'x')`,
		`INSERT INTO user_webauthn (user_id, handle) VALUES (1, x'02')`,
		`INSERT INTO user_passkeys (user_id, credential_id, credential, name, created_at) VALUES (1, x'03', '{}', 'Phone', 'x')`,
	} {
		if _, err := d.Writer.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	for _, stmt := range []string{
		`INSERT INTO user_recovery_codes (user_id, code_hash, created_at) VALUES (1, x'01', 'x')`,
		`INSERT INTO user_passkeys (user_id, credential_id, credential, name, created_at) VALUES (1, x'03', '{}', 'Laptop', 'x')`,
		`INSERT INTO user_passkeys (user_id, credential_id, credential, name, created_at) VALUES (1, x'04', '{}', '', 'x')`,
		`UPDATE users SET mfa_required = 2`,
	} {
		if _, err := d.Writer.ExecContext(ctx, stmt); err == nil {
			t.Errorf("accepted %s", stmt)
		}
	}
	if _, err := d.Writer.ExecContext(ctx, "DELETE FROM users WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := d.Reader.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM user_totp) + (SELECT count(*) FROM user_recovery_codes)
		+ (SELECT count(*) FROM user_webauthn) + (SELECT count(*) FROM user_passkeys)`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("credentials survived their user: %d %v", left, err)
	}
}
