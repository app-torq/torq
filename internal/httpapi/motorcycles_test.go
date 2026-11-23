package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

// Motorcycles (specs/006-motorcycles): the vehicle type and the catalog
// filtered by it.

func TestCatalogByVehicleType(t *testing.T) {
	_, c := catalogEnv(t)

	// Without vehicle_type the answers are the cars, as before.
	var makes catalogList[catalogNameJSON]
	c.call(http.MethodGet, "/catalog/makes?q=duc", nil, http.StatusOK, &makes)
	if len(makes.Items) != 0 {
		t.Fatalf("motorcycle make without vehicle_type: %+v", makes.Items)
	}
	c.call(http.MethodGet, "/catalog/makes?vehicle_type=motorcycle&q=duc", nil, http.StatusOK, &makes)
	if len(makes.Items) != 1 || makes.Items[0].Name != "Ducati" {
		t.Fatalf("motorcycle makes %+v", makes.Items)
	}
	var models catalogList[catalogModelJSON]
	c.call(http.MethodGet, "/catalog/models?vehicle_type=motorcycle&make=bmw", nil, http.StatusOK, &models)
	if len(models.Items) != 1 || models.Items[0].Name != "R 1200 GS" {
		t.Fatalf("models %+v", models.Items)
	}
	c.call(http.MethodGet, "/catalog/models?make=bmw", nil, http.StatusOK, &models)
	if len(models.Items) != 0 {
		t.Fatalf("car models of BMW in the fixture: %+v", models.Items)
	}

	// Motorcycles need no generation; versions carry their years.
	var engines catalogList[catalogEngineJSON]
	c.call(http.MethodGet, "/catalog/engines?vehicle_type=motorcycle&make=BMW&model=R%201200%20GS", nil, http.StatusOK, &engines)
	if len(engines.Items) != 2 || *engines.Items[1].PowerKW != 92 || *engines.Items[1].YearFrom != 2013 || *engines.Items[1].YearTo != 2018 {
		t.Fatalf("versions %+v", engines.Items)
	}
	if code := c.errorCode(http.MethodGet, "/catalog/engines?make=BMW&model=R%201200%20GS", nil, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("cars still need a generation: %s", code)
	}
	// Car engines carry the generation's years.
	c.call(http.MethodGet, "/catalog/engines?make=Tesla&model=Model%203&generation=2017%E2%80%932023", nil, http.StatusOK, &engines)
	if len(engines.Items) != 1 || *engines.Items[0].YearFrom != 2017 || *engines.Items[0].YearTo != 2023 {
		t.Fatalf("car engine years %+v", engines.Items)
	}

	for _, path := range []string{"/catalog/makes", "/catalog/models?make=x", "/catalog/generations?make=x&model=y", "/catalog/engines?make=x&model=y"} {
		sep := "?"
		if path != "/catalog/makes" {
			sep = "&"
		}
		var bad struct {
			Error struct {
				Fields map[string]string `json:"fields"`
			} `json:"error"`
		}
		c.call(http.MethodGet, path+sep+"vehicle_type=truck", nil, http.StatusUnprocessableEntity, &bad)
		if bad.Error.Fields["vehicle_type"] != "validation.invalid_choice" {
			t.Errorf("%s: %v", path, bad.Error.Fields)
		}
	}
}

func TestVehicleType(t *testing.T) {
	e := newEnv(t)
	c := e.admin()

	car := createVehicle(t, c, 1000)
	if car.VehicleType != "car" {
		t.Fatalf("default type %q", car.VehicleType)
	}
	var out vehicleResp
	c.call(http.MethodPost, "/vehicles", map[string]any{
		"vehicle_type": "motorcycle", "make": "Ducati", "model": "Monster 821", "current_odometer_km": 12000,
	}, http.StatusCreated, &out)
	bike := out.Vehicle
	if bike.VehicleType != "motorcycle" {
		t.Fatalf("created %+v", bike)
	}

	// PATCH without vehicle_type keeps it; with it, changes it.
	c.call(http.MethodPatch, "/vehicles/"+itoa(bike.ID), map[string]any{"license_plate": "MZ-12"}, http.StatusOK, &out)
	if out.Vehicle.VehicleType != "motorcycle" {
		t.Fatalf("type lost on patch: %+v", out.Vehicle)
	}
	c.call(http.MethodPatch, "/vehicles/"+itoa(car.ID), map[string]any{"vehicle_type": "motorcycle"}, http.StatusOK, &out)
	if out.Vehicle.VehicleType != "motorcycle" || out.Vehicle.Make != car.Make || out.Vehicle.CurrentOdometerKm != 1000 {
		t.Fatalf("type change touched other fields: %+v", out.Vehicle)
	}

	var bad struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	c.call(http.MethodPost, "/vehicles", map[string]any{"vehicle_type": "truck", "make": "MAN", "model": "TGX", "current_odometer_km": 1},
		http.StatusUnprocessableEntity, &bad)
	if bad.Error.Fields["vehicle_type"] != "validation.invalid_choice" {
		t.Fatalf("truck: %v", bad.Error.Fields)
	}

	// The history export names the type.
	var hist struct {
		Vehicle struct {
			VehicleType string `json:"vehicle_type"`
		} `json:"vehicle"`
	}
	c.call(http.MethodGet, "/vehicles/"+itoa(bike.ID)+"/history?download=0", nil, http.StatusOK, &hist)
	if hist.Vehicle.VehicleType != "motorcycle" {
		t.Fatalf("history %+v", hist)
	}
}

type motorcycleSpecsResp struct {
	Specs struct {
		EngineOilSpec string               `json:"engine_oil_spec"`
		Motorcycle    *motorcycleSpecsJSON `json:"motorcycle"`
	} `json:"specs"`
}

func TestMotorcycleSpecs(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	var out vehicleResp
	c.call(http.MethodPost, "/vehicles", map[string]any{
		"vehicle_type": "motorcycle", "make": "Ducati", "model": "Monster 821", "current_odometer_km": 12000,
	}, http.StatusCreated, &out)
	path := "/vehicles/" + itoa(out.Vehicle.ID) + "/specs"

	// Empty at first: the object is always there.
	var got motorcycleSpecsResp
	c.call(http.MethodGet, path, nil, http.StatusOK, &got)
	if m := got.Specs.Motorcycle; m == nil || m.FinalDrive != nil || m.ChainSize != "" {
		t.Fatalf("empty specs %+v", m)
	}

	moto := map[string]any{
		"final_drive": "chain", "sprocket_front_teeth": 15, "sprocket_rear_teeth": 43, "chain_size": " 525 ", "chain_links": 112,
		"fork_oil_spec": "SAE 7.5W", "fork_oil_capacity_ml": 491,
		"valve_clearance_intake_min_mm": 0.10, "valve_clearance_intake_max_mm": 0.15,
		"valve_clearance_exhaust_min_mm": 0.20, "valve_clearance_exhaust_max_mm": 0.25,
	}
	c.call(http.MethodPut, path, map[string]any{"engine_oil_spec": "Shell Advance 15W-50", "motorcycle": moto}, http.StatusOK, &got)
	m := got.Specs.Motorcycle
	if *m.FinalDrive != "chain" || *m.SprocketRearTeeth != 43 || m.ChainSize != "525" || *m.ChainLinks != 112 ||
		*m.ForkOilCapacityMl != 491 || *m.ValveClearanceExhaustMaxMm != 0.25 {
		t.Fatalf("saved %+v", m)
	}

	// The history export carries them (metric, as stored).
	var hist struct {
		Specs map[string]any `json:"specs"`
	}
	c.call(http.MethodGet, "/vehicles/"+itoa(out.Vehicle.ID)+"/history?download=0", nil, http.StatusOK, &hist)
	if hist.Specs["chain_links"] != float64(112) || hist.Specs["final_drive"] != "chain" || hist.Specs["fork_oil_capacity_ml"] != float64(491) {
		t.Fatalf("history specs %v", hist.Specs)
	}

	// An older client sends the sheet without "motorcycle": the values stay.
	c.call(http.MethodPut, path, map[string]any{"engine_oil_spec": "Motul 7100 15W-50"}, http.StatusOK, &got)
	if got.Specs.EngineOilSpec != "Motul 7100 15W-50" || got.Specs.Motorcycle.ChainLinks == nil || *got.Specs.Motorcycle.ChainLinks != 112 {
		t.Fatalf("older client cleared the motorcycle values: %+v", got.Specs)
	}

	// A type change keeps them too.
	c.call(http.MethodPatch, "/vehicles/"+itoa(out.Vehicle.ID), map[string]any{"vehicle_type": "car"}, http.StatusOK, nil)
	c.call(http.MethodGet, path, nil, http.StatusOK, &got)
	if got.Specs.Motorcycle.FinalDrive == nil {
		t.Fatal("type change cleared the motorcycle values")
	}

	// Validation, with prefixed field keys.
	var bad struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	c.call(http.MethodPut, path, map[string]any{"motorcycle": map[string]any{
		"final_drive": "cardan", "sprocket_front_teeth": 4, "sprocket_rear_teeth": 151, "chain_links": 301,
		"fork_oil_capacity_ml": 0, "chain_size": strings.Repeat("x", 41), "fork_oil_spec": strings.Repeat("y", 121),
		"valve_clearance_intake_min_mm": 0.2, "valve_clearance_intake_max_mm": 0.1,
		"valve_clearance_exhaust_min_mm": 2.5, "valve_clearance_exhaust_max_mm": nil,
	}}, http.StatusUnprocessableEntity, &bad)
	want := map[string]string{
		"motorcycle.final_drive": "validation.invalid_choice", "motorcycle.sprocket_front_teeth": "validation.range",
		"motorcycle.sprocket_rear_teeth": "validation.range", "motorcycle.chain_links": "validation.range",
		"motorcycle.fork_oil_capacity_ml": "validation.range", "motorcycle.chain_size": "validation.length",
		"motorcycle.fork_oil_spec": "validation.length", "motorcycle.valve_clearance_intake_max_mm": "validation.range",
		"motorcycle.valve_clearance_exhaust_min_mm": "validation.range",
	}
	for k, v := range want {
		if bad.Error.Fields[k] != v {
			t.Errorf("%s = %q, want %q", k, bad.Error.Fields[k], v)
		}
	}
	if len(bad.Error.Fields) != len(want) {
		t.Errorf("fields %v", bad.Error.Fields)
	}
	// A max without a min is refused.
	c.call(http.MethodPut, path, map[string]any{"motorcycle": map[string]any{"valve_clearance_intake_max_mm": 0.15}}, http.StatusUnprocessableEntity, &bad)
	if bad.Error.Fields["motorcycle.valve_clearance_intake_max_mm"] != "validation.range" {
		t.Fatalf("max without min: %v", bad.Error.Fields)
	}

	// An empty object clears them.
	c.call(http.MethodPut, path, map[string]any{"motorcycle": map[string]any{}}, http.StatusOK, &got)
	if m := got.Specs.Motorcycle; m.FinalDrive != nil || m.ChainLinks != nil || m.ChainSize != "" {
		t.Fatalf("not cleared %+v", m)
	}
}

func TestCategoriesByVehicleType(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	c.call(http.MethodPost, "/service-categories", map[string]any{"custom_name": "Battery", "is_fluid": false}, http.StatusCreated, nil)

	list := func(query string) map[string]categoryJSON {
		var l listResponse[categoryJSON]
		c.call(http.MethodGet, "/service-categories"+query, nil, http.StatusOK, &l)
		out := map[string]categoryJSON{}
		for _, cat := range l.Items {
			key := ""
			if cat.BuiltinKey != nil {
				key = *cat.BuiltinKey
			} else if cat.CustomName != nil {
				key = *cat.CustomName
			}
			out[key] = cat
		}
		return out
	}
	all := list("")
	if len(all) != 18 || all["cabin_filter"].VehicleType == nil || *all["cabin_filter"].VehicleType != "car" ||
		*all["fork_oil"].VehicleType != "motorcycle" || all["engine_oil"].VehicleType != nil || all["Battery"].VehicleType != nil {
		t.Fatalf("all categories: %d", len(all))
	}
	moto := list("?vehicle_type=motorcycle")
	for _, k := range []string{"chain_maintenance", "chain_sprockets", "valve_clearance", "fork_oil", "final_drive_oil", "drive_belt", "engine_oil", "spark_glow_plugs", "Battery"} {
		if _, ok := moto[k]; !ok {
			t.Errorf("motorcycle list lacks %s", k)
		}
	}
	for _, k := range []string{"cabin_filter", "differential_fluid"} {
		if _, ok := moto[k]; ok {
			t.Errorf("motorcycle list has %s", k)
		}
	}
	car := list("?vehicle_type=car")
	if _, ok := car["fork_oil"]; ok || len(car) != 12 {
		t.Errorf("car list: %d entries", len(car))
	}
	if code := c.errorCode(http.MethodGet, "/service-categories?vehicle_type=truck", nil, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Errorf("unknown type: %s", code)
	}

	// An entry keeps a car-only category after its vehicle became a motorcycle.
	v := createVehicle(t, c, 1000)
	var log struct {
		ServiceLog struct {
			ID    int64 `json:"id"`
			Items []struct {
				CategoryID int64 `json:"category_id"`
			} `json:"items"`
		} `json:"service_log"`
	}
	c.call(http.MethodPost, "/service-logs", map[string]any{
		"vehicle_id": v.ID, "service_date": "2026-10-01", "odometer_km": 1000,
		"items": []map[string]any{{"category_id": all["cabin_filter"].ID}},
	}, http.StatusCreated, &log)
	c.call(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{"vehicle_type": "motorcycle"}, http.StatusOK, nil)
	c.call(http.MethodGet, "/service-logs/"+itoa(log.ServiceLog.ID), nil, http.StatusOK, &log)
	if len(log.ServiceLog.Items) != 1 || log.ServiceLog.Items[0].CategoryID != all["cabin_filter"].ID {
		t.Fatalf("entry lost its category: %+v", log.ServiceLog)
	}
}
