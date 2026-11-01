package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/marekbeckmann/torq/internal/specfill"
)

func TestSpecMetaRoundTrip(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	base := "/vehicles/" + itoa(createVehicle(t, c, 1000).ID)

	var out struct {
		Specs specsJSON `json:"specs"`
	}
	c.call(http.MethodGet, base+"/specs", nil, http.StatusOK, &out)
	if out.Specs.Meta == nil || len(out.Specs.Meta) != 0 {
		t.Fatalf("new vehicle meta %+v", out.Specs.Meta)
	}

	c.call(http.MethodPut, base+"/specs", map[string]any{
		"engine_oil_viscosity": "5W-30", "engine_oil_spec": "MB 229.51", "tire_pressure_front_bar": 2.3,
		"meta": map[string]any{
			"engine_oil_viscosity":    map[string]any{"is_auto": true, "source_url": "https://www.carwiki.de/w203", "source_name": "carwiki.de"},
			"engine_oil_spec":         map[string]any{"is_auto": false},
			"tire_pressure_front_bar": map[string]any{"is_auto": true, "source_url": "https://adac.de/reifen", "source_name": "adac.de"},
			// Metadata of an empty field is dropped.
			"coolant_spec": map[string]any{"is_auto": true, "source_url": "https://example.com", "source_name": "example.com"},
		},
	}, http.StatusOK, &out)
	m := out.Specs.Meta
	if len(m) != 3 || !m["engine_oil_viscosity"].IsAuto || m["engine_oil_viscosity"].SourceName != "carwiki.de" ||
		m["engine_oil_spec"].IsAuto || m["tire_pressure_front_bar"].SourceURL != "https://adac.de/reifen" {
		t.Fatalf("meta %+v", m)
	}
	c.call(http.MethodGet, base+"/specs", nil, http.StatusOK, &out)
	if len(out.Specs.Meta) != 3 {
		t.Fatalf("stored meta %+v", out.Specs.Meta)
	}

	var bad struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	c.call(http.MethodPut, base+"/specs", map[string]any{
		"engine_oil_viscosity": "5W-30", "engine_oil_spec": "MB 229.51", "brake_fluid_spec": "DOT 4",
		"meta": map[string]any{
			"engine_oil_viscosity": map[string]any{"is_auto": true, "source_url": "javascript:alert(1)"},
			"engine_oil_spec":      map[string]any{"is_auto": true},
			"unknown_field":        map[string]any{"is_auto": false},
		},
	}, http.StatusUnprocessableEntity, &bad)
	f := bad.Error.Fields
	if f["meta.engine_oil_viscosity"] != "validation.url" || f["meta.engine_oil_spec"] != "validation.required" || f["meta"] != "validation.invalid_choice" {
		t.Fatalf("validation %v", f)
	}
}

type fakeLookup struct {
	got specfill.Vehicle
	out map[string]specfill.Value
	err error
}

func (f *fakeLookup) Lookup(_ context.Context, v specfill.Vehicle) (map[string]specfill.Value, error) {
	f.got = v
	return f.out, f.err
}

func TestAutofillSpecs(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	v := createVehicle(t, c, 1000)
	path := "/vehicles/" + itoa(v.ID) + "/autofill-specs"

	var me meResponse
	c.call(http.MethodGet, "/auth/me", nil, http.StatusOK, &me)
	if me.Instance.SpecAutofill {
		t.Fatal("auto-fill advertised without GEMINI_API_KEY")
	}
	if code := c.errorCode(http.MethodPost, path, nil, http.StatusForbidden); code != "specs.autofill_disabled" {
		t.Fatalf("disabled %q", code)
	}

	src, name := "https://adac.de/reifen", "adac.de"
	fake := &fakeLookup{out: map[string]specfill.Value{
		"tire_pressure_front_bar": {Value: 2.3, SourceURL: &src, SourceName: &name},
		"engine_oil_spec":         {},
	}}
	e.srv.specfill = fake
	c.call(http.MethodGet, "/auth/me", nil, http.StatusOK, &me)
	if !me.Instance.SpecAutofill {
		t.Fatal("auto-fill not advertised")
	}
	var out struct {
		Fields map[string]struct {
			Value      any     `json:"value"`
			SourceURL  *string `json:"source_url"`
			SourceName *string `json:"source_name"`
		} `json:"fields"`
	}
	c.call(http.MethodPost, path, nil, http.StatusOK, &out)
	if f := out.Fields["tire_pressure_front_bar"]; f.Value != 2.3 || *f.SourceURL != src || *f.SourceName != name {
		t.Fatalf("found %+v", f)
	}
	if f, ok := out.Fields["engine_oil_spec"]; !ok || f.Value != nil || f.SourceURL != nil {
		t.Fatalf("not found must be null %+v", f)
	}
	if fake.got.Make != v.Make || fake.got.ChassisCode != v.ChassisCode || fake.got.ModelYear != 2006 {
		t.Fatalf("vehicle passed %+v", fake.got)
	}
	// Nothing is saved until the user saves the dialog.
	var specs struct {
		Specs specsJSON `json:"specs"`
	}
	c.call(http.MethodGet, "/vehicles/"+itoa(v.ID)+"/specs", nil, http.StatusOK, &specs)
	if specs.Specs.TirePressureFrontBar != nil {
		t.Fatal("auto-fill saved values")
	}

	fake.out, fake.err = nil, specfill.ErrNoSources
	c.call(http.MethodPost, path, nil, http.StatusOK, &out)
	if len(out.Fields) != len(specfill.Keys()) || out.Fields["coolant_spec"].Value != nil {
		t.Fatalf("no sources %+v", out.Fields)
	}
	// Upstream failures say what to fix.
	for err, want := range map[error]string{
		errors.New("specfill: gemini: connection reset"):                                                 "specs.autofill_failed",
		&specfill.APIError{Status: 404, Message: "This model is no longer available"}:                    "specs.autofill_model_unavailable",
		&specfill.APIError{Status: 429, Message: "You exceeded your current quota"}:                      "specs.autofill_quota",
		&specfill.APIError{Status: 400, Message: "API key not valid. Please pass a valid key."}:          "specs.autofill_key_invalid",
		&specfill.APIError{Status: 403, Message: "Permission denied"}:                                    "specs.autofill_key_invalid",
		&specfill.APIError{Status: 503, Message: "This model is currently experiencing high demand."}:    "specs.autofill_busy",
		&specfill.APIError{Status: 500, Message: "Internal error"}:                                       "specs.autofill_failed",
		&specfill.SearchError{Status: 401, Message: "Unauthorized: missing or invalid API key."}:         "specs.autofill_search_key_invalid",
		&specfill.SearchError{Status: 432, Message: "This request exceeds your plan's set usage limit."}: "specs.autofill_search_quota",
		&specfill.SearchError{Status: 429, Message: "Too many requests"}:                                 "specs.autofill_search_quota",
		fmt.Errorf("%w: context deadline exceeded", specfill.ErrTimeout):                                 "specs.autofill_timeout",
		&specfill.SearchError{Status: 500, Message: "Internal error"}:                                    "specs.autofill_failed",
	} {
		fake.err = err
		if code := c.errorCode(http.MethodPost, path, nil, http.StatusBadGateway); code != want {
			t.Errorf("%v: code %q, want %q", err, code, want)
		}
	}
	if code := c.errorCode(http.MethodPost, "/vehicles/999/autofill-specs", nil, http.StatusNotFound); code != "not_found" {
		t.Fatalf("unknown vehicle %q", code)
	}
}

// The auto-fill needs both keys: Tavily searches, Gemini extracts.
func TestAutofillNeedsBothKeys(t *testing.T) {
	for name, keys := range map[string][2]string{"gemini only": {"g", ""}, "tavily only": {"", "t"}, "both": {"g", "t"}} {
		cfg := newEnv(t).cfg
		cfg.GeminiAPIKey, cfg.TavilyAPIKey = keys[0], keys[1]
		e := newEnvWithConfig(t, cfg)
		var me meResponse
		e.admin().call(http.MethodGet, "/auth/me", nil, http.StatusOK, &me)
		if want := name == "both"; me.Instance.SpecAutofill != want {
			t.Errorf("%s: spec_autofill %v", name, me.Instance.SpecAutofill)
		}
	}
}

func TestTyrePressureRange(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	base := "/vehicles/" + itoa(createVehicle(t, c, 1000).ID)

	var out struct {
		Specs specsJSON `json:"specs"`
	}
	c.call(http.MethodPut, base+"/specs", map[string]any{
		"tire_pressure_front_bar": 2.2, "tire_pressure_front_max_bar": 2.4, "tire_pressure_rear_bar": 2.5,
	}, http.StatusOK, &out)
	s := out.Specs
	if *s.TirePressureFrontBar != 2.2 || *s.TirePressureFrontMaxBar != 2.4 || *s.TirePressureRearBar != 2.5 || s.TirePressureRearMaxBar != nil {
		t.Fatalf("range %+v", s)
	}

	var bad struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	c.call(http.MethodPut, base+"/specs", map[string]any{
		"tire_pressure_front_bar": 2.4, "tire_pressure_front_max_bar": 2.2, // below the lower value
		"tire_pressure_rear_max_bar": 2.9, // no lower value
	}, http.StatusUnprocessableEntity, &bad)
	if bad.Error.Fields["tire_pressure_front_max_bar"] != "validation.range" || bad.Error.Fields["tire_pressure_rear_max_bar"] != "validation.range" {
		t.Fatalf("validation %v", bad.Error.Fields)
	}
}
