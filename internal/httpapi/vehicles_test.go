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
	c.call(http.MethodPatch, "/vehicles/"+itoa(v.ID), map[string]any{"vin": "SHORT"}, http.StatusUnprocessableEntity, &fields)
	if fields.Error.Fields["vin"] != "validation.vin" {
		t.Fatalf("fields %v", fields.Error.Fields)
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
}
