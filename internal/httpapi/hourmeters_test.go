package httpapi

import (
	"net/http"
	"testing"
)

type hourMeterResp struct {
	HourMeter hourMeterJSON `json:"hour_meter"`
}

// createTractor creates a vehicle tracked by hours only.
func createTractor(t *testing.T, c *client, hours float64) vehicleJSON {
	t.Helper()
	var out vehicleResp
	c.call(http.MethodPost, "/vehicles", map[string]any{
		"make": "Deutz", "model": "D 6206", "has_odometer": false,
		"hour_meters": []map[string]any{{"name": "Engine hours", "hours": hours}},
	}, http.StatusCreated, &out)
	return out.Vehicle
}

// TestHourMeters covers hour meters on vehicles (#24): hours-only vehicles,
// a meter next to the odometer, quick updates with their confirmations, the
// reading history and the rules for removing a meter.
func TestHourMeters(t *testing.T) {
	e := newEnv(t)
	c := e.admin()

	tractor := createTractor(t, c, 1250.44)
	if tractor.HasOdometer || tractor.CurrentOdometerKm != 0 || len(tractor.HourMeters) != 1 || tractor.HourMeters[0].CurrentHours != 1250.4 {
		t.Fatalf("tractor %+v", tractor)
	}
	meter := tractor.HourMeters[0]
	for _, body := range []map[string]any{
		{"make": "Deutz", "model": "D 6206", "has_odometer": false},
		{"make": "Deutz", "model": "D 6206", "has_odometer": false, "hour_meters": []map[string]any{{"name": "x", "hours": -1}}},
		{"make": "Deutz", "model": "D 6206", "has_odometer": false, "hour_meters": []map[string]any{{"name": "", "hours": 1}}},
		{"make": "Deutz", "model": "D 6206"},
	} {
		c.errorCode(http.MethodPost, "/vehicles", body, http.StatusUnprocessableEntity)
	}

	// Quick update, confirmations and history.
	var upd vehicleResp
	c.call(http.MethodPost, "/hour-meters/"+itoa(meter.ID)+"/readings", map[string]any{"hours": 1262}, http.StatusOK, &upd)
	if upd.Vehicle.HourMeters[0].CurrentHours != 1262 {
		t.Fatalf("after update %+v", upd.Vehicle.HourMeters)
	}
	if code := c.errorCode(http.MethodPost, "/hour-meters/"+itoa(meter.ID)+"/readings", map[string]any{"hours": 1126}, http.StatusConflict); code != "hour_meter.decrease_requires_confirmation" {
		t.Fatalf("decrease %q", code)
	}
	if code := c.errorCode(http.MethodPost, "/hour-meters/"+itoa(meter.ID)+"/readings", map[string]any{"hours": 9000}, http.StatusConflict); code != "hour_meter.large_jump_requires_confirmation" {
		t.Fatalf("jump %q", code)
	}
	c.errorCode(http.MethodPost, "/hour-meters/"+itoa(meter.ID)+"/readings", map[string]any{}, http.StatusUnprocessableEntity)
	var hist listResponse[hoursReadingJSON]
	c.call(http.MethodGet, "/hour-meters/"+itoa(meter.ID)+"/readings", nil, http.StatusOK, &hist)
	if hist.Total != 2 || hist.Items[0].Hours != 1262 || hist.Items[1].Source != "initial" {
		t.Fatalf("history %+v", hist.Items)
	}
	c.errorCode(http.MethodGet, "/hour-meters/999/readings", nil, http.StatusNotFound)

	// The only meter of a vehicle without an odometer can't be removed.
	if code := c.errorCode(http.MethodDelete, "/hour-meters/"+itoa(meter.ID), nil, http.StatusConflict); code != "hour_meter.required" {
		t.Fatalf("delete only meter %q", code)
	}

	// A car keeps its odometer and gets a generator meter.
	car := createVehicle(t, c, 120000)
	if !car.HasOdometer || len(car.HourMeters) != 0 {
		t.Fatalf("car %+v", car)
	}
	var gen hourMeterResp
	c.call(http.MethodPost, "/vehicles/"+itoa(car.ID)+"/hour-meters", map[string]any{"name": "Generator", "hours": 312}, http.StatusCreated, &gen)
	if code := c.errorCode(http.MethodPost, "/vehicles/"+itoa(car.ID)+"/hour-meters", map[string]any{"name": "Second", "hours": 1}, http.StatusConflict); code != "hour_meter.limit_reached" {
		t.Fatalf("second meter %q", code)
	}
	c.call(http.MethodPatch, "/hour-meters/"+itoa(gen.HourMeter.ID), map[string]any{"name": "Onan generator"}, http.StatusOK, &gen)
	if gen.HourMeter.Name != "Onan generator" || gen.HourMeter.CurrentHours != 312 {
		t.Fatalf("renamed %+v", gen.HourMeter)
	}
	var got vehicleResp
	c.call(http.MethodGet, "/vehicles/"+itoa(car.ID), nil, http.StatusOK, &got)
	if !got.Vehicle.HasOdometer || got.Vehicle.CurrentOdometerKm != 120000 || len(got.Vehicle.HourMeters) != 1 {
		t.Fatalf("car with meter %+v", got.Vehicle)
	}

	// Turning the odometer off needs a meter; the car has one now.
	plain := createVehicle(t, c, 1000)
	if code := c.errorCode(http.MethodPatch, "/vehicles/"+itoa(plain.ID), map[string]any{"has_odometer": false}, http.StatusUnprocessableEntity); code != "vehicle.tracking_required" {
		t.Fatalf("odometer off without meter %q", code)
	}
	c.call(http.MethodPatch, "/vehicles/"+itoa(car.ID), map[string]any{"has_odometer": false}, http.StatusOK, &got)
	if got.Vehicle.HasOdometer {
		t.Fatal("odometer still on")
	}
	c.call(http.MethodPatch, "/vehicles/"+itoa(car.ID), map[string]any{"model": "C 220"}, http.StatusOK, &got)
	if got.Vehicle.HasOdometer {
		t.Fatal("unrelated PATCH turned the odometer back on")
	}
	c.call(http.MethodPatch, "/vehicles/"+itoa(car.ID), map[string]any{"has_odometer": true}, http.StatusOK, &got)
	if !got.Vehicle.HasOdometer || got.Vehicle.CurrentOdometerKm != 120000 {
		t.Fatalf("odometer back on %+v", got.Vehicle)
	}
	c.call(http.MethodDelete, "/hour-meters/"+itoa(gen.HourMeter.ID), nil, http.StatusNoContent, nil)

	var list listResponse[vehicleJSON]
	c.call(http.MethodGet, "/vehicles", nil, http.StatusOK, &list)
	for _, v := range list.Items {
		if v.HourMeters == nil {
			t.Fatalf("vehicle %d without hour_meters list", v.ID)
		}
	}
}

// TestServiceLogHours covers hours on service entries: required instead of
// the odometer for hours-only vehicles, optional next to it otherwise.
func TestServiceLogHours(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	cats := categoryIDs(t, c)
	tractor := createTractor(t, c, 1250)
	meter := tractor.HourMeters[0]
	oil := []map[string]any{{"category_id": cats["engine_oil"], "cost_cents": 8900}}

	if code := c.errorCode(http.MethodPost, "/service-logs", map[string]any{"vehicle_id": tractor.ID, "service_date": "2026-10-15", "items": oil}, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("missing hours %q", code)
	}
	var created serviceLogResp
	c.call(http.MethodPost, "/service-logs", map[string]any{
		"vehicle_id": tractor.ID, "service_date": "2026-10-15", "items": oil,
		"meter_readings": []map[string]any{{"hour_meter_id": meter.ID, "hours": 1262}},
	}, http.StatusCreated, &created)
	l := created.ServiceLog
	if l.OdometerRecorded || l.OdometerKm != 0 || len(l.MeterReadings) != 1 || l.MeterReadings[0].Hours != 1262 {
		t.Fatalf("entry %+v", l)
	}
	var veh vehicleResp
	c.call(http.MethodGet, "/vehicles/"+itoa(tractor.ID), nil, http.StatusOK, &veh)
	if veh.Vehicle.HourMeters[0].CurrentHours != 1262 || veh.Vehicle.CurrentOdometerKm != 0 {
		t.Fatalf("vehicle after entry %+v", veh.Vehicle)
	}
	var odo listResponse[readingJSON]
	c.call(http.MethodGet, "/vehicles/"+itoa(tractor.ID)+"/odometer-readings", nil, http.StatusOK, &odo)
	if odo.Total != 0 {
		t.Fatalf("odometer readings for an hours-only vehicle: %+v", odo.Items)
	}

	// Editing replaces the readings; the current value is never lowered.
	c.call(http.MethodPut, "/service-logs/"+itoa(l.ID), map[string]any{
		"vehicle_id": tractor.ID, "service_date": "2026-10-15", "items": oil,
		"meter_readings": []map[string]any{{"hour_meter_id": meter.ID, "hours": 1260.5}},
	}, http.StatusOK, &created)
	if created.ServiceLog.MeterReadings[0].Hours != 1260.5 {
		t.Fatalf("edited %+v", created.ServiceLog.MeterReadings)
	}
	var hist listResponse[hoursReadingJSON]
	c.call(http.MethodGet, "/hour-meters/"+itoa(meter.ID)+"/readings", nil, http.StatusOK, &hist)
	if hist.Total != 2 {
		t.Fatalf("readings after edit %+v", hist.Items)
	}

	// Reports, timeline and history carry the hours; responses are checked against openapi.yaml.
	var costs struct {
		Hours            *float64 `json:"hours"`
		CostPerHourCents *float64 `json:"cost_per_hour_cents"`
		DistanceKm       *int64   `json:"distance_km"`
	}
	c.call(http.MethodGet, "/reports/costs?vehicle_id="+itoa(tractor.ID), nil, http.StatusOK, &costs)
	if costs.Hours == nil || *costs.Hours != 10.5 || costs.CostPerHourCents == nil || costs.DistanceKm != nil {
		t.Fatalf("costs %+v", costs)
	}
	var tl listResponse[timelineJSON]
	c.call(http.MethodGet, "/vehicles/"+itoa(tractor.ID)+"/timeline", nil, http.StatusOK, &tl)
	if len(tl.Items) != 1 || tl.Items[0].OdometerRecorded || len(tl.Items[0].MeterReadings) != 1 {
		t.Fatalf("timeline %+v", tl.Items)
	}
	c.call(http.MethodGet, "/vehicles/"+itoa(tractor.ID)+"/history", nil, http.StatusOK, nil)

	// A car with a generator: km required, hours optional; another vehicle's meter is refused.
	car := createVehicle(t, c, 120000)
	var gen hourMeterResp
	c.call(http.MethodPost, "/vehicles/"+itoa(car.ID)+"/hour-meters", map[string]any{"name": "Generator", "hours": 312}, http.StatusCreated, &gen)
	c.errorCode(http.MethodPost, "/service-logs", map[string]any{"vehicle_id": car.ID, "service_date": "2026-10-15", "items": oil,
		"meter_readings": []map[string]any{{"hour_meter_id": gen.HourMeter.ID, "hours": 318}}}, http.StatusUnprocessableEntity)
	c.errorCode(http.MethodPost, "/service-logs", map[string]any{"vehicle_id": car.ID, "service_date": "2026-10-15", "odometer_km": 121000, "items": oil,
		"meter_readings": []map[string]any{{"hour_meter_id": meter.ID, "hours": 318}}}, http.StatusUnprocessableEntity)
	c.call(http.MethodPost, "/service-logs", map[string]any{"vehicle_id": car.ID, "service_date": "2026-10-15", "odometer_km": 121000, "items": oil}, http.StatusCreated, &created)
	if !created.ServiceLog.OdometerRecorded || len(created.ServiceLog.MeterReadings) != 0 {
		t.Fatalf("car entry %+v", created.ServiceLog)
	}
	c.call(http.MethodPost, "/service-logs", map[string]any{"vehicle_id": car.ID, "service_date": "2026-10-16", "odometer_km": 121500, "items": oil,
		"meter_readings": []map[string]any{{"hour_meter_id": gen.HourMeter.ID, "hours": 318.5}}}, http.StatusCreated, &created)
	if created.ServiceLog.MeterReadings[0].Hours != 318.5 {
		t.Fatalf("generator hours %+v", created.ServiceLog.MeterReadings)
	}

	// Deleting the entry removes its readings, so the meter can be removed again.
	if code := c.errorCode(http.MethodDelete, "/hour-meters/"+itoa(gen.HourMeter.ID), nil, http.StatusConflict); code != "hour_meter.in_use" {
		t.Fatalf("delete used meter %q", code)
	}
	c.call(http.MethodDelete, "/service-logs/"+itoa(created.ServiceLog.ID), nil, http.StatusOK, nil)
	c.call(http.MethodDelete, "/hour-meters/"+itoa(gen.HourMeter.ID), nil, http.StatusNoContent, nil)
}

// TestRepairHours covers hours when completing and reopening a project.
func TestRepairHours(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	tractor := createTractor(t, c, 1250)
	meter := tractor.HourMeters[0]
	r := newRepair(t, c, tractor.ID, "Clutch", nil)

	if code := c.errorCode(http.MethodPost, "/repairs/"+itoa(r.ID)+"/status", map[string]any{"status": "completed", "completed_on": "2026-10-20"}, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("missing hours %q", code)
	}
	var got repairResp
	c.call(http.MethodPost, "/repairs/"+itoa(r.ID)+"/status", map[string]any{
		"status": "completed", "completed_on": "2026-10-20", "completion_meter_readings": []map[string]any{{"hour_meter_id": meter.ID, "hours": 1270}},
	}, http.StatusOK, &got)
	if got.Repair.CompletionOdometerRecorded || len(got.Repair.CompletionMeterReadings) != 1 || got.Repair.CompletionMeterReadings[0].Hours != 1270 {
		t.Fatalf("completed %+v", got.Repair)
	}
	c.call(http.MethodPatch, "/repairs/"+itoa(r.ID), map[string]any{"completion_meter_readings": []map[string]any{{"hour_meter_id": meter.ID, "hours": 1271}}}, http.StatusOK, &got)
	if got.Repair.CompletionMeterReadings[0].Hours != 1271 {
		t.Fatalf("edited %+v", got.Repair.CompletionMeterReadings)
	}
	c.call(http.MethodPatch, "/repairs/"+itoa(r.ID), map[string]any{"title": "Clutch disc"}, http.StatusOK, &got)
	if len(got.Repair.CompletionMeterReadings) != 1 {
		t.Fatalf("readings lost on unrelated edit %+v", got.Repair)
	}
	var list listResponse[repairSummaryJSON]
	c.call(http.MethodGet, "/repairs?vehicle_id="+itoa(tractor.ID), nil, http.StatusOK, &list)
	if len(list.Items[0].CompletionMeterReadings) != 1 {
		t.Fatalf("list %+v", list.Items)
	}
	c.call(http.MethodPost, "/repairs/"+itoa(r.ID)+"/status", map[string]any{"status": "in_progress"}, http.StatusOK, &got)
	if len(got.Repair.CompletionMeterReadings) != 0 {
		t.Fatalf("reopened %+v", got.Repair)
	}
}

type reminderResp struct {
	Reminder reminderJSON `json:"reminder"`
}

// TestHoursReminders covers reminders by hours: every 250 h or 12 months
// with a 25 h lead, the state moving with hour updates and service entries,
// mark done, and refusing a distance interval without an odometer.
func TestHoursReminders(t *testing.T) {
	e := newEnv(t)
	c := e.admin()
	cats := categoryIDs(t, c)
	tractor := createTractor(t, c, 1250)
	meter := tractor.HourMeters[0]
	path := "/hour-meters/" + itoa(meter.ID) + "/readings"

	// Distance intervals, and meters of other vehicles, are refused.
	car := createVehicle(t, c, 120000)
	for _, body := range []map[string]any{
		{"vehicle_id": tractor.ID, "title": "Grease", "interval_km": 1000},
		{"vehicle_id": car.ID, "title": "Grease", "interval_hours": 50, "hour_meter_id": meter.ID},
		{"vehicle_id": tractor.ID, "title": "Grease", "interval_hours": 50},
		{"vehicle_id": tractor.ID, "title": "Grease", "interval_hours": 50, "lead_hours": 50, "hour_meter_id": meter.ID},
	} {
		if code := c.errorCode(http.MethodPost, "/reminders", body, http.StatusUnprocessableEntity); code != "validation.failed" {
			t.Fatalf("%v: %q", body, code)
		}
	}

	// A linked oil reminder: the category's distance default is skipped, its months default applies.
	var rem reminderResp
	c.call(http.MethodPost, "/reminders", map[string]any{
		"vehicle_id": tractor.ID, "service_category_id": cats["engine_oil"], "hour_meter_id": meter.ID, "interval_hours": 250,
	}, http.StatusCreated, &rem)
	r := rem.Reminder
	if r.IntervalKm != nil || r.IntervalMonths == nil || *r.LeadHours != 25 || *r.StartHours != 1250 || *r.NextDueHours != 1500 || r.State != "upcoming" {
		t.Fatalf("created %+v", r)
	}

	// The last oil change at 1,262 h moves the due point to 1,512 h.
	c.call(http.MethodPost, "/service-logs", map[string]any{
		"vehicle_id": tractor.ID, "service_date": "2026-10-15", "items": []map[string]any{{"category_id": cats["engine_oil"]}},
		"meter_readings": []map[string]any{{"hour_meter_id": meter.ID, "hours": 1262}},
	}, http.StatusCreated, nil)
	c.call(http.MethodGet, "/reminders/"+itoa(r.ID), nil, http.StatusOK, &rem)
	if *rem.Reminder.NextDueHours != 1512 || *rem.Reminder.LastDoneHours != 1262 || *rem.Reminder.Baseline.Hours != 1262 {
		t.Fatalf("after service %+v", rem.Reminder)
	}

	var upd struct {
		Reminders []reminderJSON `json:"reminders"`
	}
	c.call(http.MethodPost, path, map[string]any{"hours": 1490}, http.StatusOK, &upd)
	if got := upd.Reminders[0]; got.State != "due_soon" || *got.StateTrigger != "hours" || *got.RemainingHours != 22 {
		t.Fatalf("1490 h %+v", got)
	}
	c.call(http.MethodPost, path, map[string]any{"hours": 1512}, http.StatusOK, &upd)
	if got := upd.Reminders[0]; got.State != "overdue" || *got.RemainingHours != 0 {
		t.Fatalf("1512 h %+v", got)
	}

	// A standalone reminder marked done at a given hour count.
	c.call(http.MethodPost, "/reminders", map[string]any{
		"vehicle_id": tractor.ID, "title": "Grease front axle", "hour_meter_id": meter.ID, "interval_hours": 50, "lead_hours": 5,
	}, http.StatusCreated, &rem)
	var done struct {
		Reminder reminderJSON `json:"reminder"`
		Vehicle  vehicleJSON  `json:"vehicle"`
	}
	c.call(http.MethodPost, "/reminders/"+itoa(rem.Reminder.ID)+"/done", map[string]any{
		"meter_readings": []map[string]any{{"hour_meter_id": meter.ID, "hours": 1515}},
	}, http.StatusOK, &done)
	if *done.Reminder.LastDoneHours != 1515 || *done.Reminder.NextDueHours != 1565 || done.Vehicle.HourMeters[0].CurrentHours != 1515 {
		t.Fatalf("done %+v", done.Reminder)
	}

	// PATCH without hours fields (an older client) keeps them; null removes them.
	c.call(http.MethodPatch, "/reminders/"+itoa(rem.Reminder.ID), map[string]any{"title": "Grease axles", "interval_months": 6}, http.StatusOK, &rem)
	if rem.Reminder.IntervalHours == nil || *rem.Reminder.IntervalHours != 50 || rem.Reminder.Title != "Grease axles" {
		t.Fatalf("patch kept hours %+v", rem.Reminder)
	}
	c.call(http.MethodPatch, "/reminders/"+itoa(rem.Reminder.ID), map[string]any{"title": "Grease axles", "interval_months": 6, "hour_meter_id": nil}, http.StatusOK, &rem)
	if rem.Reminder.IntervalHours != nil || rem.Reminder.HourMeterID != nil || rem.Reminder.NextDueHours != nil {
		t.Fatalf("patch removed hours %+v", rem.Reminder)
	}
}
