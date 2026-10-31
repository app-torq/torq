package catalog_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/marekbeckmann/torq/internal/catalog"
	"github.com/marekbeckmann/torq/internal/catalog/catalogtest"
	"github.com/marekbeckmann/torq/internal/db"
	"github.com/marekbeckmann/torq/internal/store"
	"github.com/marekbeckmann/torq/internal/testutil"
)

func fixtureDB(t *testing.T) (*db.DB, *store.Queries) {
	t.Helper()
	d := testutil.NewDB(t)
	catalogtest.Load(t, d)
	return d, store.New(d.Reader)
}

func TestKBALookup(t *testing.T) {
	ctx := context.Background()
	_, q := fixtureDB(t)

	upper, err := catalog.KBA(ctx, q, "9002", "1A2")
	if err != nil {
		t.Fatal(err)
	}
	lower, err := catalog.KBA(ctx, q, "9002", "1a2")
	if err != nil {
		t.Fatal(err)
	}
	if len(upper) != 1 || len(lower) != 1 || upper[0] != lower[0] {
		t.Fatalf("case-insensitive TSN: %+v vs %+v", upper, lower)
	}
	if v := upper[0]; v.Make != "Škoda" || v.PowerKw.Int64 != 110 || v.FuelType.String != "diesel" || v.BodyCode != "AC" {
		t.Fatalf("variant %+v", v)
	}

	multi, err := catalog.KBA(ctx, q, "9001", "AAB")
	if err != nil {
		t.Fatal(err)
	}
	if len(multi) != 2 || multi[0].TradeName != "C 220 CDI" || multi[1].TradeName != "CLC 220 CDI" {
		t.Fatalf("multi %+v", multi)
	}

	none, err := catalog.KBA(ctx, q, "0000", "XXX")
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("unknown key %v %v", none, err)
	}
}

func TestCascade(t *testing.T) {
	ctx := context.Background()
	_, q := fixtureDB(t)

	makes, err := catalog.Makes(ctx, q, "merc")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(makes, []string{"Mercedes-AMG", "Mercedes-Benz"}) {
		t.Fatalf("makes %v", makes)
	}
	if m, _ := catalog.Makes(ctx, q, "CITROEN"); !slices.Equal(m, []string{"Citroën"}) {
		t.Fatalf("accent-insensitive %v", m)
	}
	// Substring matches anywhere; prefix matches sort first.
	if m, _ := catalog.Makes(ctx, q, "es"); len(m) != 3 || m[0] != "Mercedes-AMG" {
		t.Fatalf("substring %v", m)
	}
	if m, _ := catalog.Makes(ctx, q, "t"); len(m) == 0 || m[0] != "Tesla" {
		t.Fatalf("prefix first %v", m)
	}
	if m, _ := catalog.Makes(ctx, q, ""); len(m) != 4 {
		t.Fatalf("all makes %v", m)
	}

	models, err := catalog.Models(ctx, q, "mercedes benz", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0].Name != "C-Klasse" || models[0].YearFrom != 2000 || models[0].YearTo != 2007 ||
		models[1].Name != "C-Klasse T-Modell" {
		t.Fatalf("models %+v", models)
	}
	if m, _ := catalog.Models(ctx, q, "Mercedes-Benz", "t modell"); len(m) != 1 {
		t.Fatalf("model query %+v", m)
	}

	gens, err := catalog.Generations(ctx, q, "Mercedes-Benz", "c-klasse", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(gens) != 2 || gens[0].Name != "2004–2007" || gens[1].Name != "2000–2004" || gens[1].YearFrom != 2000 {
		t.Fatalf("generations (newest first) %+v", gens)
	}
	if g, _ := catalog.Generations(ctx, q, "Citroën", "C5", ""); len(g) != 1 || g[0].YearTo != 0 {
		t.Fatalf("open-ended generation %+v", g)
	}

	engines, err := catalog.Engines(ctx, q, "Mercedes-Benz", "C-Klasse", "2000–2004", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(engines) != 2 || engines[0].Name != "220 CDI 5AT RWD" || engines[0].PowerKw.Int64 != 105 ||
		engines[0].DisplacementCc.Int64 != 2148 || engines[0].FuelType.String != "diesel" {
		t.Fatalf("engines %+v", engines)
	}
	if e, _ := catalog.Engines(ctx, q, "Tesla", "Model 3", "2017–2023", "awd"); len(e) != 1 || e[0].DisplacementCc.Valid {
		t.Fatalf("electric engine %+v", e)
	}

	for name, rows := range map[string]int{
		"models":      lenOf(catalog.Models(ctx, q, "Trabbi Custom", "")),
		"generations": lenOf(catalog.Generations(ctx, q, "Mercedes-Benz", "Unknown", "")),
		"engines":     lenOf(catalog.Engines(ctx, q, "Mercedes-Benz", "C-Klasse", "1950", "")),
	} {
		if rows != 0 {
			t.Errorf("%s for unknown parent: %d rows", name, rows)
		}
	}
}

func lenOf[T any](rows []T, err error) int {
	if err != nil {
		return -1
	}
	return len(rows)
}

// shippedDB imports the real embedded catalog once for the plan and speed
// checks (research C9).
func shippedDB(t *testing.T) *db.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("imports the full catalog")
	}
	d := testutil.NewDB(t)
	if _, err := catalog.Seed(context.Background(), d, catalog.EmbeddedSeed, time.Now()); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestQueriesUseIndexes(t *testing.T) {
	d := shippedDB(t)
	plans := map[string][]any{
		"SELECT * FROM catalog_kba WHERE hsn = ? AND tsn = ? ORDER BY trade_name, year_from, power_kw, id": {"0710", "472"},
		"SELECT model FROM catalog_vehicles WHERE make_key = ? AND instr(model_key, ?) > 0":                {"mercedesbenz", "c"},
		"SELECT generation FROM catalog_vehicles WHERE make_key = ? AND model_key = ?":                     {"volkswagen", "golf"},
		"SELECT engine FROM catalog_vehicles WHERE make_key = ? AND model_key = ? AND generation_key = ?":  {"bmw", "3series", "x"},
		"SELECT make FROM catalog_vehicles WHERE instr(make_key, ?) > 0":                                   {"bm"},
	}
	for query, args := range plans {
		rows, err := d.Reader.Query("EXPLAIN QUERY PLAN "+query, args...)
		if err != nil {
			t.Fatal(err)
		}
		var plan []string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			plan = append(plan, detail)
		}
		_ = rows.Close()
		joined := strings.Join(plan, "; ")
		if !strings.Contains(joined, "USING INDEX") && !strings.Contains(joined, "USING COVERING INDEX") {
			t.Errorf("%s\n  plan: %s", query, joined)
		}
	}
}

func TestLookupSpeed(t *testing.T) {
	d := shippedDB(t)
	ctx := context.Background()
	q := store.New(d.Reader)
	var keys []store.CatalogKba
	if err := func() error {
		rows, err := d.Reader.Query("SELECT hsn, tsn FROM catalog_kba ORDER BY id LIMIT 200")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var k store.CatalogKba
			if err := rows.Scan(&k.Hsn, &k.Tsn); err != nil {
				return err
			}
			keys = append(keys, k)
		}
		return rows.Err()
	}(); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 200 {
		t.Fatalf("need 200 keys, got %d", len(keys))
	}
	durations := make([]time.Duration, 0, len(keys))
	for _, k := range keys {
		start := time.Now()
		if rows, err := catalog.KBA(ctx, q, k.Hsn, strings.ToLower(k.Tsn)); err != nil || len(rows) == 0 {
			t.Fatalf("lookup %s/%s: %v %d", k.Hsn, k.Tsn, err, len(rows))
		}
		durations = append(durations, time.Since(start))
	}
	slices.Sort(durations)
	if median := durations[len(durations)/2]; median >= 5*time.Millisecond {
		t.Fatalf("median lookup %v, want < 5ms", median)
	}
}

func BenchmarkKBALookup(b *testing.B) {
	d := testutil.NewDB(b)
	if _, err := catalog.Seed(context.Background(), d, catalog.EmbeddedSeed, time.Now()); err != nil {
		b.Fatal(err)
	}
	q := store.New(d.Reader)
	ctx := context.Background()
	for b.Loop() {
		if _, err := catalog.KBA(ctx, q, "0710", "472"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkModels(b *testing.B) {
	d := testutil.NewDB(b)
	if _, err := catalog.Seed(context.Background(), d, catalog.EmbeddedSeed, time.Now()); err != nil {
		b.Fatal(err)
	}
	q := store.New(d.Reader)
	ctx := context.Background()
	for b.Loop() {
		if _, err := catalog.Models(ctx, q, "Volkswagen", "go"); err != nil {
			b.Fatal(err)
		}
	}
}
