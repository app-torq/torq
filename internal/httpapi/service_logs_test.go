package httpapi

import (
	"net/http"
	"testing"
)

type serviceLogResp struct {
	ServiceLog serviceLogJSON `json:"service_log"`
	Reminders  []reminderJSON `json:"reminders"`
	Warnings   []string       `json:"warnings"`
}

func categoryIDs(t *testing.T, c *client) map[string]int64 {
	t.Helper()
	var list listResponse[categoryJSON]
	c.call(http.MethodGet, "/service-categories", nil, http.StatusOK, &list)
	out := map[string]int64{}
	for _, cat := range list.Items {
		if cat.BuiltinKey != nil {
			out[*cat.BuiltinKey] = cat.ID
		}
	}
	if len(out) != 11 {
		t.Fatalf("expected 11 built-in categories, got %d", len(out))
	}
	return out
}

func TestServiceLogContract(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	v := createVehicle(t, c, 214500)
	cats := categoryIDs(t, c)

	var rem struct {
		Reminder reminderJSON `json:"reminder"`
	}
	c.call(http.MethodPost, "/reminders", map[string]any{
		"vehicle_id": v.ID, "service_category_id": cats["engine_oil"], "interval_km": 10000, "interval_months": 12,
		"start_date": "2026-08-01", "start_odometer_km": 205500,
	}, http.StatusCreated, &rem)
	if rem.Reminder.State != "due_soon" || *rem.Reminder.RemainingKm != 1000 || rem.Reminder.Baseline.Source != "start" {
		t.Fatalf("reminder %+v", rem.Reminder)
	}

	body := map[string]any{
		"vehicle_id": v.ID, "service_date": "2026-10-15", "odometer_km": 215000,
		"items": []map[string]any{
			{"category_id": cats["engine_oil"], "brand": "Mobil 1", "specification": "MB 229.51", "quantity_ml": 6500, "quantity_unit": "L", "oem_part_number": "A6511800109", "cost_cents": 8950},
			{"category_id": cats["air_filter"], "cost_cents": 2490},
			{"category_id": cats["cabin_filter"], "cost_cents": 1990},
		},
	}
	var created serviceLogResp
	c.call(http.MethodPost, "/service-logs", body, http.StatusCreated, &created)
	if len(created.ServiceLog.Items) != 3 || created.ServiceLog.TotalCents != 13430 {
		t.Fatalf("log %+v", created.ServiceLog)
	}
	if created.ServiceLog.Items[0].CostType != "fluids" || created.ServiceLog.Items[1].CostType != "parts" {
		t.Fatalf("cost type defaults: %+v", created.ServiceLog.Items)
	}
	if len(created.Reminders) != 1 || created.Reminders[0].State != "upcoming" || *created.Reminders[0].NextDueKm != 225000 || *created.Reminders[0].NextDueDate != "2027-10-15" {
		t.Fatalf("reminders %+v", created.Reminders)
	}
	var veh vehicleResp
	c.call(http.MethodGet, "/vehicles/"+itoa(v.ID), nil, http.StatusOK, &veh)
	if veh.Vehicle.CurrentOdometerKm != 215000 {
		t.Fatalf("odometer %d", veh.Vehicle.CurrentOdometerKm)
	}

	// Validation codes.
	dup := map[string]any{"vehicle_id": v.ID, "service_date": "2026-10-16", "odometer_km": 215000, "items": []map[string]any{{"category_id": cats["air_filter"]}, {"category_id": cats["air_filter"]}}}
	if code := c.errorCode(http.MethodPost, "/service-logs", dup, http.StatusUnprocessableEntity); code != "service_log.duplicate_category" {
		t.Fatalf("code %q", code)
	}
	empty := map[string]any{"vehicle_id": v.ID, "service_date": "2026-10-16", "odometer_km": 215000, "items": []any{}}
	if code := c.errorCode(http.MethodPost, "/service-logs", empty, http.StatusUnprocessableEntity); code != "service_log.items_required" {
		t.Fatalf("code %q", code)
	}
	future := map[string]any{"vehicle_id": v.ID, "service_date": "2027-01-01", "odometer_km": 215000, "items": []map[string]any{{"category_id": cats["air_filter"]}}}
	if code := c.errorCode(http.MethodPost, "/service-logs", future, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("code %q", code)
	}

	// Archived custom category is rejected on create; in-use category cannot be deleted.
	var custom struct {
		Category categoryJSON `json:"category"`
	}
	c.call(http.MethodPost, "/service-categories", map[string]any{"custom_name": "DPF Additive", "is_fluid": true}, http.StatusCreated, &custom)
	c.call(http.MethodPost, "/service-logs", map[string]any{"vehicle_id": v.ID, "service_date": "2026-09-01", "odometer_km": 213000, "items": []map[string]any{{"category_id": custom.Category.ID, "quantity_ml": 1000, "quantity_unit": "mL"}}}, http.StatusCreated, nil)
	if code := c.errorCode(http.MethodDelete, "/service-categories/"+itoa(custom.Category.ID), nil, http.StatusConflict); code != "category.in_use" {
		t.Fatalf("code %q", code)
	}
	c.call(http.MethodPost, "/service-categories/"+itoa(custom.Category.ID)+"/archive", nil, http.StatusOK, nil)
	arch := map[string]any{"vehicle_id": v.ID, "service_date": "2026-10-16", "odometer_km": 215000, "items": []map[string]any{{"category_id": custom.Category.ID}}}
	if code := c.errorCode(http.MethodPost, "/service-logs", arch, http.StatusUnprocessableEntity); code != "service_log.category_archived" {
		t.Fatalf("code %q", code)
	}
	if code := c.errorCode(http.MethodPost, "/service-categories/"+itoa(cats["engine_oil"])+"/archive", nil, http.StatusConflict); code != "category.builtin_readonly" {
		t.Fatalf("code %q", code)
	}

	// Inconsistent history produces a warning but saves.
	var warned serviceLogResp
	c.call(http.MethodPost, "/service-logs", map[string]any{"vehicle_id": v.ID, "service_date": "2026-10-01", "odometer_km": 216000, "items": []map[string]any{{"category_id": cats["fuel_filter"]}}}, http.StatusCreated, &warned)
	if len(warned.Warnings) != 1 || warned.Warnings[0] != "odometer.inconsistent_with_history" {
		t.Fatalf("warnings %v", warned.Warnings)
	}

	// PUT replaces items.
	var replaced serviceLogResp
	body["items"] = []map[string]any{{"category_id": cats["engine_oil"], "cost_cents": 9000}}
	c.call(http.MethodPut, "/service-logs/"+itoa(created.ServiceLog.ID), body, http.StatusOK, &replaced)
	if len(replaced.ServiceLog.Items) != 1 || replaced.ServiceLog.TotalCents != 9000 {
		t.Fatalf("replaced %+v", replaced.ServiceLog)
	}

	// Filters.
	var list listResponse[serviceLogJSON]
	c.call(http.MethodGet, "/service-logs?vehicle_id="+itoa(v.ID)+"&category_id="+itoa(cats["engine_oil"]), nil, http.StatusOK, &list)
	if list.Total != 1 {
		t.Fatalf("category filter total %d", list.Total)
	}
	c.call(http.MethodGet, "/service-logs?q=A65118", nil, http.StatusOK, &list)
	if list.Total != 0 {
		t.Fatalf("part number search should not find replaced item, got %d", list.Total)
	}
	c.call(http.MethodGet, "/service-logs?from=2026-10-01&to=2026-10-31", nil, http.StatusOK, &list)
	if list.Total != 2 {
		t.Fatalf("date filter total %d", list.Total)
	}

	// DELETE returns re-evaluated reminders falling back to the start baseline.
	var del struct {
		Reminders []reminderJSON `json:"reminders"`
	}
	c.call(http.MethodDelete, "/service-logs/"+itoa(created.ServiceLog.ID), nil, http.StatusOK, &del)
	if len(del.Reminders) != 1 || del.Reminders[0].Baseline.Source != "start" || del.Reminders[0].State != "overdue" {
		t.Fatalf("after delete %+v", del.Reminders)
	}
}

func TestRemindersAndDashboard(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	v := createVehicle(t, c, 100000)

	var rem struct {
		Reminder reminderJSON `json:"reminder"`
	}
	c.call(http.MethodPost, "/reminders", map[string]any{"vehicle_id": v.ID, "title": "Inspect CV boots", "interval_km": 20000, "start_odometer_km": 81000}, http.StatusCreated, &rem)
	if rem.Reminder.State != "due_soon" || *rem.Reminder.LeadKm != 1000 {
		t.Fatalf("reminder %+v", rem.Reminder)
	}
	if code := c.errorCode(http.MethodPost, "/reminders", map[string]any{"vehicle_id": v.ID, "title": "x"}, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("code %q", code)
	}

	var dash struct {
		Vehicles  []vehicleJSON  `json:"vehicles"`
		Attention []reminderJSON `json:"attention"`
	}
	c.call(http.MethodGet, "/dashboard", nil, http.StatusOK, &dash)
	if len(dash.Attention) != 1 || dash.Vehicles[0].ReminderCounts.DueSoon != 1 {
		t.Fatalf("dashboard %+v", dash)
	}

	var done struct {
		Reminder reminderJSON `json:"reminder"`
		Vehicle  vehicleJSON  `json:"vehicle"`
	}
	c.call(http.MethodPost, "/reminders/"+itoa(rem.Reminder.ID)+"/done", map[string]any{}, http.StatusOK, &done)
	if done.Reminder.State != "upcoming" || done.Reminder.Baseline.Source != "mark_done" {
		t.Fatalf("done %+v", done.Reminder)
	}

	c.call(http.MethodPost, "/reminders/"+itoa(rem.Reminder.ID)+"/pause", nil, http.StatusOK, &rem)
	if !rem.Reminder.Paused {
		t.Fatal("not paused")
	}
	var list listResponse[reminderJSON]
	c.call(http.MethodGet, "/reminders", nil, http.StatusOK, &list)
	if len(list.Items) != 0 {
		t.Fatal("paused reminder listed by default")
	}
	c.call(http.MethodGet, "/reminders?include_paused=true", nil, http.StatusOK, &list)
	if len(list.Items) != 1 {
		t.Fatal("paused reminder missing with include_paused")
	}
	c.call(http.MethodPatch, "/reminders/"+itoa(rem.Reminder.ID), map[string]any{"title": "Inspect CV boots", "interval_km": 15000, "lead_km": 500}, http.StatusOK, &rem)
	if *rem.Reminder.IntervalKm != 15000 || *rem.Reminder.NextDueKm != 115000 {
		t.Fatalf("patched %+v", rem.Reminder)
	}
	c.call(http.MethodDelete, "/reminders/"+itoa(rem.Reminder.ID), nil, http.StatusNoContent, nil)
}
