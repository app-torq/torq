package httpapi

import (
	"net/http"
	"testing"
)

type vehicleResp struct {
	Vehicle vehicleJSON `json:"vehicle"`
}

func createVehicle(t *testing.T, c *client, km int64) vehicleJSON {
	t.Helper()
	var out vehicleResp
	c.call(http.MethodPost, "/vehicles", map[string]any{
		"make": "Mercedes-Benz", "model": "C 220 CDI", "chassis_code": "S203", "model_year": 2006,
		"engine_code": "OM646", "current_odometer_km": km,
	}, http.StatusCreated, &out)
	return out.Vehicle
}

func TestVehicleCRUD(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	v := createVehicle(t, c, 214500)
	if v.CurrentOdometerKm != 214500 || v.ChassisCode != "S203" || v.AvatarURL != nil {
		t.Fatalf("created %+v", v)
	}

	var got vehicleResp
	c.call(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{"license_plate": "B-TQ 646", "vin": "wdb2030081a123456"}, http.StatusOK, &got)
	if got.Vehicle.LicensePlate == nil || *got.Vehicle.LicensePlate != "B-TQ 646" || got.Vehicle.Make != "Mercedes-Benz" || *got.Vehicle.VIN != "WDB2030081A123456" {
		t.Fatalf("patched %+v", got.Vehicle)
	}

	var fields struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	for _, bad := range []string{"WDB2030081A1234567890123456789012", "JZA80 0012345", "ÄBC"} {
		c.call(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{"vin": bad}, http.StatusUnprocessableEntity, &fields)
		if fields.Error.Fields["vin"] != "validation.vin" {
			t.Fatalf("vin %q: fields %v", bad, fields.Error.Fields)
		}
	}

	var list listResponse[vehicleJSON]
	c.call(http.MethodPost, "/vehicles/"+itoa(v.ID)+"/archive", nil, http.StatusOK, nil)
	c.call(http.MethodGet, "/vehicles", nil, http.StatusOK, &list)
	if len(list.Items) != 0 {
		t.Fatal("archived vehicle listed as active")
	}
	c.call(http.MethodGet, "/vehicles?archived=all", nil, http.StatusOK, &list)
	if len(list.Items) != 1 || !list.Items[0].Archived {
		t.Fatalf("all: %+v", list.Items)
	}
	c.call(http.MethodPost, "/vehicles/"+itoa(v.ID)+"/unarchive", nil, http.StatusOK, nil)
	c.call(http.MethodGet, "/vehicles?archived=true", nil, http.StatusOK, &list)
	if len(list.Items) != 0 {
		t.Fatal("unarchived vehicle still archived")
	}

	if code := c.errorCode(http.MethodDelete, "/vehicles/"+itoa(v.ID), map[string]string{"confirm_name": "BMW E46"}, http.StatusUnprocessableEntity); code != "vehicle.confirm_name_mismatch" {
		t.Fatalf("code %q", code)
	}
	c.call(http.MethodDelete, "/vehicles/"+itoa(v.ID), map[string]string{"confirm_name": "mercedes-benz c 220 cdi"}, http.StatusNoContent, nil)
	c.errorCode(http.MethodGet, "/vehicles/"+itoa(v.ID), nil, http.StatusNotFound)
}

// TestVehicleNonStandardVIN covers chassis numbers and old VINs that are not
// 17 characters long (issue #10).
func TestVehicleNonStandardVIN(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	v := createVehicle(t, c, 1000)
	for in, want := range map[string]string{"jza80-0012345": "JZA80-0012345", " 1F02H1234 ": "1F02H1234", "X": "X"} {
		var got vehicleResp
		c.call(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{"vin": in}, http.StatusOK, &got)
		if got.Vehicle.VIN == nil || *got.Vehicle.VIN != want {
			t.Fatalf("vin %q stored as %v", in, got.Vehicle.VIN)
		}
	}
	var got vehicleResp
	c.call(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{"vin": nil}, http.StatusOK, &got)
	if got.Vehicle.VIN != nil {
		t.Fatalf("vin not cleared: %v", *got.Vehicle.VIN)
	}
}

func TestVehicleOdometerUnitAndPower(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	v := createVehicle(t, c, 1000)
	if v.OdometerUnit != "km" || v.PowerKW != nil || v.PowerW != nil {
		t.Fatalf("defaults %+v", v)
	}
	var got vehicleResp
	// 301 hp = 224,459 W; whole kW alone would read back as 300 hp.
	c.call(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{"odometer_unit": "mi", "power_w": 224459}, http.StatusOK, &got)
	if got.Vehicle.OdometerUnit != "mi" || *got.Vehicle.PowerW != 224459 || *got.Vehicle.PowerKW != 224 {
		t.Fatalf("patched %+v", got.Vehicle)
	}
	// A client that only knows power_kw sends back the rounded value: keep power_w.
	c.call(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{"power_kw": 224, "model": "C 220"}, http.StatusOK, &got)
	if *got.Vehicle.PowerW != 224459 {
		t.Fatalf("precise power lost: %+v", got.Vehicle)
	}
	c.call(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{"power_kw": 110}, http.StatusOK, &got)
	if *got.Vehicle.PowerW != 110000 || *got.Vehicle.PowerKW != 110 {
		t.Fatalf("power_kw change: %+v", got.Vehicle)
	}
	c.call(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{"power_kw": nil}, http.StatusOK, &got)
	if got.Vehicle.PowerW != nil || got.Vehicle.PowerKW != nil {
		t.Fatalf("power not cleared: %+v", got.Vehicle)
	}
	if code := c.errorCode(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{"odometer_unit": "furlong"}, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("code %q", code)
	}
	if code := c.errorCode(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{"power_w": 2000001}, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("code %q", code)
	}
	var created vehicleResp
	c.call(http.MethodPost, "/vehicles", map[string]any{
		"make": "Toyota", "model": "Supra", "current_odometer_km": 80467, "odometer_unit": "mi", "power_kw": 206,
	}, http.StatusCreated, &created)
	if created.Vehicle.OdometerUnit != "mi" || *created.Vehicle.PowerW != 206000 {
		t.Fatalf("created %+v", created.Vehicle)
	}
}

// TestVehicleDisplayUnits checks the optional per-vehicle displacement and
// power units (#18): null follows the user's preference, PATCH keeps them
// unless sent, and null clears them.
func TestVehicleDisplayUnits(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	v := createVehicle(t, c, 1000)
	if v.DisplacementUnit != nil || v.PowerUnit != nil {
		t.Fatalf("defaults %+v", v)
	}
	var got vehicleResp
	c.call(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{"displacement_unit": "ci", "power_unit": "hp"}, http.StatusOK, &got)
	if got.Vehicle.DisplacementUnit == nil || *got.Vehicle.DisplacementUnit != "ci" || got.Vehicle.PowerUnit == nil || *got.Vehicle.PowerUnit != "hp" {
		t.Fatalf("patched %+v", got.Vehicle)
	}
	c.call(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{"model": "C 220"}, http.StatusOK, &got)
	if got.Vehicle.DisplacementUnit == nil || got.Vehicle.PowerUnit == nil {
		t.Fatalf("units lost on unrelated patch: %+v", got.Vehicle)
	}
	c.call(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{"displacement_unit": nil, "power_unit": ""}, http.StatusOK, &got)
	if got.Vehicle.DisplacementUnit != nil || got.Vehicle.PowerUnit != nil {
		t.Fatalf("units not cleared: %+v", got.Vehicle)
	}
	for _, body := range []map[string]any{{"displacement_unit": "m3"}, {"power_unit": "bhp"}} {
		if code := c.errorCode(http.MethodPatch, "/vehicles/"+itoa(v.ID), body, http.StatusUnprocessableEntity); code != "validation.failed" {
			t.Fatalf("%v: code %q", body, code)
		}
	}
	var created vehicleResp
	c.call(http.MethodPost, "/vehicles", map[string]any{
		"make": "Toyota", "model": "Supra", "current_odometer_km": 80467, "displacement_unit": "cc", "power_unit": "ps",
	}, http.StatusCreated, &created)
	if *created.Vehicle.DisplacementUnit != "cc" || *created.Vehicle.PowerUnit != "ps" {
		t.Fatalf("created %+v", created.Vehicle)
	}
	var h struct {
		Vehicle struct {
			DisplacementUnit *string `json:"displacement_unit"`
			PowerUnit        *string `json:"power_unit"`
		} `json:"vehicle"`
	}
	c.call(http.MethodGet, "/vehicles/"+itoa(created.Vehicle.ID)+"/history", nil, http.StatusOK, &h)
	if h.Vehicle.DisplacementUnit == nil || *h.Vehicle.DisplacementUnit != "cc" || h.Vehicle.PowerUnit == nil || *h.Vehicle.PowerUnit != "ps" {
		t.Fatalf("history %+v", h.Vehicle)
	}
}

func TestOdometerEndpoint(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	v := createVehicle(t, c, 214500)
	var out struct {
		Vehicle   vehicleJSON `json:"vehicle"`
		Reminders []any       `json:"reminders"`
	}
	c.call(http.MethodPost, "/vehicles/"+itoa(v.ID)+"/odometer", map[string]any{"odometer_km": 215200}, http.StatusOK, &out)
	if out.Vehicle.CurrentOdometerKm != 215200 || out.Reminders == nil {
		t.Fatalf("out %+v", out)
	}
	if code := c.errorCode(http.MethodPost, "/vehicles/"+itoa(v.ID)+"/odometer", map[string]any{"odometer_km": 21520}, http.StatusConflict); code != "odometer.decrease_requires_confirmation" {
		t.Fatalf("code %q", code)
	}
	if code := c.errorCode(http.MethodPost, "/vehicles/"+itoa(v.ID)+"/odometer", map[string]any{"odometer_km": 999999}, http.StatusConflict); code != "odometer.large_jump_requires_confirmation" {
		t.Fatalf("code %q", code)
	}
	var readings listResponse[readingJSON]
	c.call(http.MethodGet, "/vehicles/"+itoa(v.ID)+"/odometer-readings", nil, http.StatusOK, &readings)
	if len(readings.Items) != 2 || readings.Items[0].OdometerKm != 215200 {
		t.Fatalf("readings %+v", readings.Items)
	}
	if code := c.errorCode(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{"current_odometer_km": 1}, http.StatusBadRequest); code != "vehicle.odometer_use_quick_update" {
		t.Fatalf("code %q", code)
	}
}

func TestSpecSheet(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	v := createVehicle(t, c, 1000)
	base := "/vehicles/" + itoa(v.ID)

	var specs struct {
		Specs   specsJSON       `json:"specs"`
		Entries []specEntryJSON `json:"entries"`
	}
	c.call(http.MethodPut, base+"/specs", map[string]any{
		"engine_oil_viscosity": "5W-30", "engine_oil_spec": "MB 229.51", "engine_oil_capacity_ml": 6500,
		"wheel_lug_torque_nm": 150, "tire_pressure_front_bar": 2.3, "tire_pressure_rear_bar": 2.5,
	}, http.StatusOK, &specs)
	if specs.Specs.EngineOilCapacityMl == nil || *specs.Specs.EngineOilCapacityMl != 6500 || *specs.Specs.TirePressureRearBar != 2.5 {
		t.Fatalf("specs %+v", specs.Specs)
	}
	var fields struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	c.call(http.MethodPut, base+"/specs", map[string]any{"tire_pressure_front_bar": 12, "coolant_capacity_ml": 0}, http.StatusUnprocessableEntity, &fields)
	if fields.Error.Fields["tire_pressure_front_bar"] != "validation.range" || fields.Error.Fields["coolant_capacity_ml"] != "validation.positive" {
		t.Fatalf("fields %v", fields.Error.Fields)
	}

	var a, b struct {
		Entry specEntryJSON `json:"entry"`
	}
	c.call(http.MethodPost, base+"/spec-entries", map[string]string{"label": "Oil drain plug", "value": "30 Nm"}, http.StatusCreated, &a)
	c.call(http.MethodPost, base+"/spec-entries", map[string]string{"label": "Serpentine belt routing"}, http.StatusCreated, &b)
	c.call(http.MethodPut, base+"/spec-entries/order", map[string]any{"ids": []int64{b.Entry.ID, a.Entry.ID}}, http.StatusOK, &specs)
	if specs.Entries[0].ID != b.Entry.ID || specs.Entries[1].SortOrder != 2 {
		t.Fatalf("order %+v", specs.Entries)
	}
	if code := c.errorCode(http.MethodPut, base+"/spec-entries/order", map[string]any{"ids": []int64{a.Entry.ID}}, http.StatusUnprocessableEntity); code != "spec_entries.order_mismatch" {
		t.Fatalf("code %q", code)
	}
	c.call(http.MethodPatch, "/spec-entries/"+itoa(a.Entry.ID), map[string]string{"value": "25 Nm"}, http.StatusOK, &a)
	if a.Entry.Value != "25 Nm" || a.Entry.Label != "Oil drain plug" {
		t.Fatalf("patched %+v", a.Entry)
	}
	c.call(http.MethodDelete, "/spec-entries/"+itoa(a.Entry.ID), nil, http.StatusNoContent, nil)
	c.call(http.MethodGet, base+"/specs", nil, http.StatusOK, &specs)
	if len(specs.Entries) != 1 {
		t.Fatalf("entries %+v", specs.Entries)
	}

	// Spec entries take PDFs as well as images (#27).
	if rec := c.upload("/spec-entries/"+itoa(b.Entry.ID)+"/attachments", "belt-routing.pdf", testPDF, ""); rec.Code != http.StatusCreated {
		t.Fatalf("pdf upload: %d %s", rec.Code, rec.Body)
	}
	c.call(http.MethodGet, base+"/specs", nil, http.StatusOK, &specs)
	if got := specs.Entries[0].Attachments; len(got) != 1 || got[0].MimeType != "application/pdf" {
		t.Fatalf("attachments %+v", got)
	}
}

// TestVehicleProfileExtension covers the HSN/TSN and technical fields from
// specs/002-vehicle-catalog-lookup (FR-020, FR-021).
func TestVehicleProfileExtension(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	var created vehicleResp
	c.call(http.MethodPost, "/vehicles", map[string]any{
		"make": "Mercedes-Benz", "model": "C 220 CDI", "current_odometer_km": 1000,
		"hsn": "0710", "tsn": "abc", "displacement_cc": 2148, "power_kw": 110, "fuel_type": "diesel",
	}, http.StatusCreated, &created)
	v := created.Vehicle
	if v.HSN == nil || *v.HSN != "0710" || v.TSN == nil || *v.TSN != "ABC" || v.DisplacementCC == nil || *v.DisplacementCC != 2148 ||
		v.PowerKW == nil || *v.PowerKW != 110 || v.FuelType == nil || *v.FuelType != "diesel" {
		t.Fatalf("created %+v", v)
	}

	// PATCH keeps untouched fields; "" and null clear a field.
	var got vehicleResp
	c.call(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{"tsn": "", "power_kw": nil}, http.StatusOK, &got)
	if got.Vehicle.TSN != nil || got.Vehicle.PowerKW != nil || got.Vehicle.HSN == nil || *got.Vehicle.HSN != "0710" ||
		got.Vehicle.DisplacementCC == nil {
		t.Fatalf("patched %+v", got.Vehicle)
	}

	// HSN and TSN are independent.
	c.call(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{"hsn": nil, "tsn": " 1a2 "}, http.StatusOK, &got)
	if got.Vehicle.HSN != nil || got.Vehicle.TSN == nil || *got.Vehicle.TSN != "1A2" {
		t.Fatalf("independent %+v", got.Vehicle)
	}

	cases := []struct {
		field string
		value any
		code  string
	}{
		{"hsn", "071", "vehicle.hsn_invalid"},
		{"hsn", "07A0", "vehicle.hsn_invalid"},
		{"tsn", "AB", "vehicle.tsn_invalid"},
		{"tsn", "A-B", "vehicle.tsn_invalid"},
		{"displacement_cc", 0, "vehicle.displacement_out_of_range"},
		{"displacement_cc", 20001, "vehicle.displacement_out_of_range"},
		{"power_kw", 0, "vehicle.power_out_of_range"},
		{"power_kw", 2001, "vehicle.power_out_of_range"},
		{"fuel_type", "kerosene", "vehicle.fuel_type_invalid"},
	}
	for _, tc := range cases {
		var fields struct {
			Error struct {
				Code   string            `json:"code"`
				Fields map[string]string `json:"fields"`
			} `json:"error"`
		}
		c.call(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{tc.field: tc.value}, http.StatusUnprocessableEntity, &fields)
		if fields.Error.Code != "validation.failed" || fields.Error.Fields[tc.field] != tc.code {
			t.Errorf("%s=%v: %+v", tc.field, tc.value, fields.Error)
		}
	}
}
