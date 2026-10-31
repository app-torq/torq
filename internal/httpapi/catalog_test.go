package httpapi

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/marekbeckmann/torq/internal/catalog"
	"github.com/marekbeckmann/torq/internal/catalog/catalogtest"
)

type catalogList[T any] struct {
	Items []T `json:"items"`
}

func catalogEnv(t *testing.T) (*env, *client) {
	t.Helper()
	e := newEnv(t)
	c := e.admin()
	catalogtest.Load(t, e.db)
	return e, c
}

func TestCatalogKBAContract(t *testing.T) {
	e, c := catalogEnv(t)

	rec := c.do(http.MethodGet, "/catalog/kba?hsn=9002&tsn=1a2", nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "private, no-cache" {
		t.Fatalf("status %d cache %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	var raw catalogList[map[string]any]
	c.call(http.MethodGet, "/catalog/kba?hsn=9002&tsn=1a2", nil, http.StatusOK, &raw)
	if len(raw.Items) != 1 {
		t.Fatalf("items %+v", raw.Items)
	}
	keys := make([]string, 0)
	for k := range raw.Items[0] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"body_code", "displacement_cc", "engine_code", "fuel_type", "generation", "make", "match", "model", "power_kw", "trade_name", "year_from"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("fields %v", keys)
	}

	var got catalogList[kbaVariantJSON]
	c.call(http.MethodGet, "/catalog/kba?hsn=9002&tsn=1A2", nil, http.StatusOK, &got)
	v := got.Items[0]
	if v.Make != "Škoda" || *v.PowerKW != 110 || *v.DisplacementCC != 1968 || *v.FuelType != "diesel" || *v.YearFrom != 2013 {
		t.Fatalf("variant %+v", v)
	}
	c.call(http.MethodGet, "/catalog/kba?hsn=9001&tsn=aab", nil, http.StatusOK, &got)
	if len(got.Items) != 2 || got.Items[0].PowerKW != nil || got.Items[0].FuelType != nil {
		t.Fatalf("multi %+v", got.Items)
	}
	c.call(http.MethodGet, "/catalog/kba?hsn=0000&tsn=XXX", nil, http.StatusOK, &raw)
	if raw.Items == nil || len(raw.Items) != 0 {
		t.Fatalf("unknown key %+v", raw.Items)
	}

	var bad struct {
		Error struct {
			Code   string            `json:"code"`
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	c.call(http.MethodGet, "/catalog/kba?hsn=071&tsn=a!b", nil, http.StatusUnprocessableEntity, &bad)
	if bad.Error.Code != "validation.failed" || bad.Error.Fields["hsn"] != "catalog.hsn_invalid" || bad.Error.Fields["tsn"] != "catalog.tsn_invalid" {
		t.Fatalf("validation %+v", bad.Error)
	}

	if code := e.client().errorCode(http.MethodGet, "/catalog/kba?hsn=9002&tsn=1A2", nil, http.StatusUnauthorized); code != "auth.unauthenticated" {
		t.Fatalf("anonymous code %q", code)
	}

	// An empty or broken catalog answers 200 [] instead of an error.
	if _, err := e.db.Writer.Exec("DROP TABLE catalog_kba"); err != nil {
		t.Fatal(err)
	}
	c.call(http.MethodGet, "/catalog/kba?hsn=9002&tsn=1A2", nil, http.StatusOK, &raw)
	if len(raw.Items) != 0 {
		t.Fatalf("broken catalog %+v", raw.Items)
	}
}

func TestCatalogCascadeContract(t *testing.T) {
	_, c := catalogEnv(t)

	var makes catalogList[catalogNameJSON]
	c.call(http.MethodGet, "/catalog/makes?q=merc", nil, http.StatusOK, &makes)
	if len(makes.Items) != 2 || makes.Items[1].Name != "Mercedes-Benz" {
		t.Fatalf("makes %+v", makes.Items)
	}
	c.call(http.MethodGet, "/catalog/makes", nil, http.StatusOK, &makes)
	if len(makes.Items) != 4 {
		t.Fatalf("all makes %+v", makes.Items)
	}

	// Names follow the lang parameter, else the caller's locale (en).
	var models catalogList[catalogModelJSON]
	c.call(http.MethodGet, "/catalog/models?make=mercedes%20benz", nil, http.StatusOK, &models)
	if len(models.Items) != 1 || models.Items[0].Name != "C-Class" || *models.Items[0].YearFrom != 1990 {
		t.Fatalf("models %+v", models.Items)
	}
	c.call(http.MethodGet, "/catalog/models?make=mercedes%20benz&lang=de", nil, http.StatusOK, &models)
	if len(models.Items) != 1 || models.Items[0].Name != "C-Klasse" {
		t.Fatalf("German models %+v", models.Items)
	}
	var kba catalogList[kbaVariantJSON]
	c.call(http.MethodGet, "/catalog/kba?hsn=9001&tsn=AAA&lang=de", nil, http.StatusOK, &kba)
	if len(kba.Items) != 1 || kba.Items[0].Model != "C-Klasse" {
		t.Fatalf("German KBA model %+v", kba.Items)
	}

	var gens catalogList[catalogGenerationJSON]
	c.call(http.MethodGet, "/catalog/generations?make=Citro%C3%ABn&model=C5", nil, http.StatusOK, &gens)
	if len(gens.Items) != 1 || gens.Items[0].Name != "2008–2017" || gens.Items[0].YearTo != nil {
		t.Fatalf("generations %+v", gens.Items)
	}
	c.call(http.MethodGet, "/catalog/generations?make=Mercedes-Benz&model=C-Klasse&lang=de", nil, http.StatusOK, &gens)
	if len(gens.Items) != 3 || gens.Items[0].Name != "S203" || gens.Items[0].Body != "estate" || gens.Items[2].Name != "Coupé (1990–1993)" {
		t.Fatalf("chassis generations %+v", gens.Items)
	}

	var engines catalogList[catalogEngineJSON]
	c.call(http.MethodGet, "/catalog/engines?make=Mercedes-Benz&model=C-Class&generation=W203&q=6mt", nil, http.StatusOK, &engines)
	if len(engines.Items) != 1 || *engines.Items[0].PowerKW != 105 || *engines.Items[0].FuelType != "diesel" {
		t.Fatalf("engines %+v", engines.Items)
	}

	// Unknown parents are not errors.
	c.call(http.MethodGet, "/catalog/models?make=Trabbi%20Custom", nil, http.StatusOK, &models)
	if len(models.Items) != 0 {
		t.Fatalf("unknown parent %+v", models.Items)
	}

	for path, field := range map[string]string{
		"/catalog/models":                          "make",
		"/catalog/generations?make=BMW":            "model",
		"/catalog/engines?make=BMW&model=3+Series": "generation",
	} {
		var bad struct {
			Error struct {
				Fields map[string]string `json:"fields"`
			} `json:"error"`
		}
		c.call(http.MethodGet, path, nil, http.StatusUnprocessableEntity, &bad)
		if bad.Error.Fields[field] != "validation.required" {
			t.Errorf("%s: %+v", path, bad.Error.Fields)
		}
	}
}

func TestCatalogMetaForMembers(t *testing.T) {
	e := newEnv(t)
	adm := e.admin()

	var meta catalogMetaJSON
	adm.call(http.MethodGet, "/catalog/meta", nil, http.StatusOK, &meta)
	if meta.ImportedAt != nil || meta.Datasets == nil || len(meta.Datasets) != 0 || meta.Update.State != "idle" {
		t.Fatalf("before import %+v", meta)
	}

	catalogtest.Load(t, e.db)
	adm.call(http.MethodPost, "/users", map[string]string{
		"username": "lena", "display_name": "Lena", "password": testPassword, "role": "member",
	}, http.StatusCreated, nil)
	member := e.login("lena", testPassword)
	member.call(http.MethodGet, "/catalog/meta", nil, http.StatusOK, &meta)
	if meta.ImportedAt == nil || *meta.ImportedAt != "2026-10-31T12:00:00Z" || len(meta.Datasets) != 1 ||
		meta.Datasets[0].ID != "fixture" || meta.Datasets[0].Attribution != "Test data" || meta.Counts.KBA != 4 || meta.Counts.Vehicles != 8 ||
		meta.Source != "bundled" {
		t.Fatalf("meta %+v", meta)
	}
}

func TestCatalogUpdate(t *testing.T) {
	e := newEnv(t)
	adm := e.admin()
	release := make(chan struct{})
	runs := 0
	e.srv.catalogUpdate.run = func(ctx context.Context) (catalog.Result, error) {
		runs++
		<-release
		if runs == 1 {
			return catalog.Result{}, errors.New("catalog update: https://example.invalid: HTTP 503")
		}
		return catalog.Result{Imported: true}, nil
	}
	wait := func() {
		t.Helper()
		e.srv.catalogUpdate.mu.Lock()
		done := e.srv.catalogUpdate.done
		e.srv.catalogUpdate.mu.Unlock()
		<-done
	}

	var started struct {
		Update catalogUpdateJSON `json:"update"`
	}
	adm.call(http.MethodPost, "/catalog/update", nil, http.StatusAccepted, &started)
	if started.Update.State != "running" || started.Update.StartedAt == nil {
		t.Fatalf("started %+v", started.Update)
	}
	if code := adm.errorCode(http.MethodPost, "/catalog/update", nil, http.StatusConflict); code != "catalog.update_running" {
		t.Fatalf("second start %q", code)
	}
	close(release)
	wait()
	var meta catalogMetaJSON
	adm.call(http.MethodGet, "/catalog/meta", nil, http.StatusOK, &meta)
	if meta.Update.State != "failed" || meta.Update.Error == nil || !strings.Contains(*meta.Update.Error, "HTTP 503") || !meta.Update.Enabled {
		t.Fatalf("failed update %+v", meta.Update)
	}
	adm.call(http.MethodPost, "/catalog/update", nil, http.StatusAccepted, nil)
	wait()
	adm.call(http.MethodGet, "/catalog/meta", nil, http.StatusOK, &meta)
	if meta.Update.State != "succeeded" || meta.Update.Error != nil || meta.Update.FinishedAt == nil {
		t.Fatalf("succeeded update %+v", meta.Update)
	}

	// Members may read the state but not start an update.
	adm.call(http.MethodPost, "/users", map[string]string{
		"username": "lena", "display_name": "Lena", "password": testPassword, "role": "member",
	}, http.StatusCreated, nil)
	if code := e.login("lena", testPassword).errorCode(http.MethodPost, "/catalog/update", nil, http.StatusForbidden); code == "" {
		t.Fatal("member started an update")
	}

	// Self-hosters can switch online updates off.
	e.srv.Config.CatalogOnlineUpdates = false
	if code := adm.errorCode(http.MethodPost, "/catalog/update", nil, http.StatusForbidden); code != "catalog.updates_disabled" {
		t.Fatalf("disabled %q", code)
	}
	adm.call(http.MethodGet, "/catalog/meta", nil, http.StatusOK, &meta)
	if meta.Update.Enabled {
		t.Fatal("meta reports updates enabled")
	}
	if runs != 2 {
		t.Fatalf("runs %d", runs)
	}
}
