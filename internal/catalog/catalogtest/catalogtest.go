// Package catalogtest provides a small catalog fixture for tests in other
// packages. Its values are illustrative only; the shipped data comes from
// internal/catalog/catalogsync.
package catalogtest

import (
	"context"
	"testing"
	"time"

	"github.com/app-torq/torq/internal/catalog"
	"github.com/app-torq/torq/internal/db"
)

func i64(v int64) *int64   { return &v }
func str(v string) *string { return &v }

// Doc is a small hand-written catalog used by the tests. Its values are
// illustrative only; the shipped data comes from internal/catalog/catalogsync.
func Doc() catalog.SeedDoc {
	return catalog.SeedDoc{
		Header: catalog.Header{
			GeneratedAt: "2026-10-31T00:00:00Z",
			License:     "ODbL-1.0",
			Datasets: []catalog.Dataset{{
				ID: "fixture", Name: "Test fixture", Publisher: "torq tests", URI: "https://example.invalid/fixture",
				Retrieved: "2026-10-31", License: "Open Database License 1.0", LicenseID: "ODbL-1.0",
				LicenseURL: "https://opendatacommons.org/licenses/odbl/1-0/", Attribution: "Test data", Modified: true,
			}},
		},
		KBA: []catalog.KBARow{
			{HSN: "9001", TSN: "AAA", Make: "Mercedes-Benz", Model: "C-Class", ModelDE: "C-Klasse", TradeName: "C 220 CDI", Source: "fz6", Match: "unique"},
			{HSN: "9001", TSN: "AAB", Make: "Mercedes-Benz", Model: "C-Class", ModelDE: "C-Klasse", TradeName: "C 220 CDI", Source: "fz6", Match: "unique"},
			{HSN: "9001", TSN: "AAB", Make: "Mercedes-Benz", Model: "CLC 220 CDI", TradeName: "CLC 220 CDI", Source: "fz6", Match: "none"},
			{HSN: "9002", TSN: "1A2", Make: "Škoda", Model: "Octavia", TradeName: "OCTAVIA", DisplacementCC: i64(1968),
				PowerKW: i64(110), FuelType: str("diesel"), BodyCode: "AC", YearFrom: i64(2013), Source: "sv42", Match: "none"},
		},
		Vehicles: []catalog.VehicleRow{
			{Make: "Mercedes-Benz", Model: "C-Class", ModelDE: "C-Klasse", Generation: "W203", YearFrom: i64(2000), YearTo: i64(2004),
				Engine: "220 CDI 5AT RWD", DisplacementCC: i64(2148), PowerKW: i64(105), FuelType: str("diesel")},
			{Make: "Mercedes-Benz", Model: "C-Class", ModelDE: "C-Klasse", Generation: "W203", YearFrom: i64(2000), YearTo: i64(2004),
				Engine: "220 CDI 6MT RWD", DisplacementCC: i64(2148), PowerKW: i64(105), FuelType: str("diesel")},
			{Make: "Mercedes-Benz", Model: "C-Class", ModelDE: "C-Klasse", Generation: "W203", YearFrom: i64(2004), YearTo: i64(2007),
				Engine: "220 CDI 5AT RWD", DisplacementCC: i64(2148), PowerKW: i64(110), FuelType: str("diesel")},
			{Make: "Mercedes-Benz", Model: "C-Class", ModelDE: "C-Klasse", Generation: "S203", YearFrom: i64(2004), YearTo: i64(2007),
				Body: "estate", Engine: "200 Kompressor 5AT RWD", DisplacementCC: i64(1796), PowerKW: i64(120), FuelType: str("petrol")},
			{Make: "Mercedes-Benz", Model: "C-Class", ModelDE: "C-Klasse", Generation: "Coupe (1990–1993)", GenerationDE: "Coupé (1990–1993)",
				YearFrom: i64(1990), YearTo: i64(1993), Engine: "190 E 2.3-16", DisplacementCC: i64(2299), PowerKW: i64(130), FuelType: str("petrol")},
			{Make: "Mercedes-AMG", Model: "C-Class", ModelDE: "C-Klasse", Generation: "S205", YearFrom: i64(2015), YearTo: i64(2018),
				Body: "estate", Engine: "C 63 AMG", DisplacementCC: i64(3982), PowerKW: i64(350), FuelType: str("petrol")},
			{Make: "Citroën", Model: "C5", Generation: "2008–2017", YearFrom: i64(2008),
				Engine: "2.0 HDi", DisplacementCC: i64(1997), PowerKW: i64(100), FuelType: str("diesel")},
			{Make: "Tesla", Model: "Model 3", Generation: "2017–2023", YearFrom: i64(2017), YearTo: i64(2023),
				Engine: "Long Range AWD", PowerKW: i64(324), FuelType: str("electric")},
		},
	}
}

// Seed encodes Doc.
func Seed(t testing.TB) []byte {
	t.Helper()
	b, err := catalog.EncodeSeed(Doc())
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Load imports Doc into d.
func Load(t testing.TB, d *db.DB) {
	t.Helper()
	if _, err := catalog.Seed(context.Background(), d, Seed(t), time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
}
