package catalog_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/marekbeckmann/torq/internal/catalog"
	"github.com/marekbeckmann/torq/internal/catalog/catalogtest"
	"github.com/marekbeckmann/torq/internal/db"
	"github.com/marekbeckmann/torq/internal/store"
	"github.com/marekbeckmann/torq/internal/testutil"
)

func ptr[T any](v T) *T { return &v }

var t0 = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

func counts(t *testing.T, d *db.DB) (kba, veh int) {
	t.Helper()
	if err := d.Reader.QueryRow("SELECT (SELECT count(*) FROM catalog_kba), (SELECT count(*) FROM catalog_vehicles)").Scan(&kba, &veh); err != nil {
		t.Fatal(err)
	}
	return kba, veh
}

func TestSeedImportsOnceAndReplacesOnChange(t *testing.T) {
	ctx := context.Background()
	d := testutil.NewDB(t)

	res, err := catalog.Seed(ctx, d, catalogtest.Seed(t), t0)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Imported || res.KBA != 4 || res.Vehicles != 7 {
		t.Fatalf("first import %+v", res)
	}
	if k, v := counts(t, d); k != 4 || v != 7 {
		t.Fatalf("rows %d %d", k, v)
	}
	var makeKey string
	if err := d.Reader.QueryRow("SELECT make_key FROM catalog_vehicles WHERE make = 'Citroën'").Scan(&makeKey); err != nil || makeKey != "citroen" {
		t.Fatalf("make_key %q %v", makeKey, err)
	}
	meta, err := store.New(d.Reader).GetCatalogMeta(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if meta.ImportedAt != "2026-10-31T12:00:00Z" || len(meta.SeedSha256) != 64 || !strings.Contains(meta.HeaderJson, `"id":"fixture"`) ||
		strings.Contains(meta.HeaderJson, `"kba":[`) {
		t.Fatalf("meta %+v", meta)
	}

	// Same bytes: no-op.
	res, err = catalog.Seed(ctx, d, catalogtest.Seed(t), t0.Add(time.Hour))
	if err != nil || res.Imported {
		t.Fatalf("second import %+v %v", res, err)
	}
	if m2, _ := store.New(d.Reader).GetCatalogMeta(ctx); m2.ImportedAt != meta.ImportedAt {
		t.Fatal("no-op import changed imported_at")
	}

	// Changed seed: all rows replaced.
	doc := catalogtest.Doc()
	doc.KBA = doc.KBA[:1]
	doc.Vehicles = doc.Vehicles[:2]
	changed, err := catalog.EncodeSeed(doc)
	if err != nil {
		t.Fatal(err)
	}
	if res, err = catalog.Seed(ctx, d, changed, t0.Add(2*time.Hour)); err != nil || !res.Imported {
		t.Fatalf("changed import %+v %v", res, err)
	}
	if k, v := counts(t, d); k != 1 || v != 2 {
		t.Fatalf("rows after replace %d %d", k, v)
	}
}

func TestSeedRejectsBadSeedAndKeepsCatalog(t *testing.T) {
	ctx := context.Background()
	d := testutil.NewDB(t)
	if _, err := catalog.Seed(ctx, d, catalogtest.Seed(t), t0); err != nil {
		t.Fatal(err)
	}
	before, _ := store.New(d.Reader).GetCatalogMeta(ctx)

	bad := map[string]func(*catalog.SeedDoc){
		"short tsn":     func(doc *catalog.SeedDoc) { doc.KBA[0].TSN = "ab" },
		"lower tsn":     func(doc *catalog.SeedDoc) { doc.KBA[0].TSN = "abc" },
		"bad hsn":       func(doc *catalog.SeedDoc) { doc.KBA[0].HSN = "07A0" },
		"bad fuel":      func(doc *catalog.SeedDoc) { doc.Vehicles[0].FuelType = ptr("kerosene") },
		"zero power":    func(doc *catalog.SeedDoc) { doc.Vehicles[0].PowerKW = ptr[int64](0) },
		"huge cc":       func(doc *catalog.SeedDoc) { doc.KBA[3].DisplacementCC = ptr[int64](20001) },
		"bad source":    func(doc *catalog.SeedDoc) { doc.KBA[0].Source = "web" },
		"missing make":  func(doc *catalog.SeedDoc) { doc.Vehicles[1].Make = "" },
		"missing trade": func(doc *catalog.SeedDoc) { doc.KBA[1].TradeName = "" },
		"symbol engine": func(doc *catalog.SeedDoc) { doc.Vehicles[1].Engine = "--" },
	}
	for name, mutate := range bad {
		doc := catalogtest.Doc()
		doc.Vehicles = append(doc.Vehicles, catalog.VehicleRow{Make: "Extra", Model: "Row", Engine: "X"}) // forces a hash change
		mutate(&doc)
		seed, err := catalog.EncodeSeed(doc)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := catalog.Seed(ctx, d, seed, t0.Add(time.Hour)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	var notGzip bytes.Buffer
	notGzip.WriteString("not a gzip stream")
	var badJSON bytes.Buffer
	zw := gzip.NewWriter(&badJSON)
	_, _ = zw.Write([]byte(`{"schema": "torq.catalog-seed/v1", "kba": [`))
	_ = zw.Close()
	var wrongSchema bytes.Buffer
	zw = gzip.NewWriter(&wrongSchema)
	_, _ = zw.Write([]byte(`{"schema": "other/v9", "kba": [], "vehicles": []}`))
	_ = zw.Close()
	for name, seed := range map[string][]byte{"not gzip": notGzip.Bytes(), "truncated json": badJSON.Bytes(), "wrong schema": wrongSchema.Bytes(), "empty": nil} {
		if _, err := catalog.Seed(ctx, d, seed, t0.Add(time.Hour)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	if k, v := counts(t, d); k != 4 || v != 7 {
		t.Fatalf("previous catalog changed: %d %d", k, v)
	}
	if after, _ := store.New(d.Reader).GetCatalogMeta(ctx); after != before {
		t.Fatalf("meta changed: %+v", after)
	}
}
