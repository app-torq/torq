package httpapi

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/app-torq/torq/internal/auth"
)

// roleUser creates a user in a new role with the given permissions (all
// vehicles) and returns the signed-in client.
func roleUser(t *testing.T, e *env, adm *client, name string, perms []auth.Permission) *client {
	t.Helper()
	keys := make([]string, 0, len(perms))
	for _, p := range perms {
		keys = append(keys, string(p))
	}
	var role struct {
		Role roleJSON `json:"role"`
	}
	adm.call(http.MethodPost, "/roles", map[string]any{"name": name, "permissions": keys}, http.StatusCreated, &role)
	adm.call(http.MethodPost, "/users", map[string]any{"username": strings.ToLower(name), "display_name": name, "password": testPassword, "role_id": role.Role.ID}, http.StatusCreated, nil)
	return e.login(strings.ToLower(name), testPassword)
}

// without returns every permission except the given ones.
func without(drop ...auth.Permission) []auth.Permission {
	var out []auth.Permission
	for _, p := range auth.AllPermissions() {
		if !slices.Contains(drop, p) {
			out = append(out, p)
		}
	}
	return out
}

// TestHiddenFieldsNeverLeave walks every route that reads a vehicle's data as
// a role that may change everything but sees no costs, notes, attachments,
// VIN or plate: no response may contain one of the marked values (SC-001).
func TestHiddenFieldsNeverLeave(t *testing.T) {
	e, adm, _, _, _, a, _ := accessUsers(t)
	c := roleUser(t, e, adm, "Helper", without(auth.PermCostsView, auth.PermNotesView, auth.PermAttachmentsView, auth.PermVehicleIDsView))
	markers := []string{"43219", "77713", "Alpha notes", "Alpha procedure", "Alpha.pdf", "Alpha-PLATE", "AVWZZZ1KZAW123456"}

	// Lists and garage-wide reads.
	paths := []string{"/vehicles?archived=all", "/dashboard", "/service-logs", "/repairs?status=planned&status=in_progress&status=waiting_on_parts&status=completed", "/reminders"}
	for _, r := range accessRoutes {
		if method, _, _ := strings.Cut(r.pattern, " "); method != http.MethodGet || r.path == nil {
			continue
		}
		paths = append(paths, r.path(a))
	}
	for _, p := range paths {
		rec := c.do(http.MethodGet, p, nil)
		for _, m := range markers {
			if strings.Contains(rec.Body.String(), m) {
				t.Errorf("GET %s (%d) shows %q", p, rec.Code, m)
			}
		}
	}
	// Attachment files and the cost report are refused outright.
	if code := c.errorCode(http.MethodGet, "/attachments/"+itoa(a.logFile), nil, http.StatusForbidden); code != "auth.forbidden" {
		t.Errorf("attachment: %q", code)
	}
	if code := c.errorCode(http.MethodGet, "/reports/costs?from=2026-01-01&to=2026-12-31", nil, http.StatusForbidden); code != "auth.forbidden" {
		t.Errorf("cost report: %q", code)
	}
	// Notes don't show through the search either.
	var logs listResponse[serviceLogJSON]
	c.call(http.MethodGet, "/service-logs?q=Alpha+notes", nil, http.StatusOK, &logs)
	if logs.Total != 0 {
		t.Errorf("search found %d entries by their notes", logs.Total)
	}
	// The administrator still sees everything.
	rec := adm.do(http.MethodGet, "/vehicles/"+itoa(a.vehicle)+"/history?format=json&download=0", nil)
	for _, m := range []string{"43219", "Alpha notes", "Alpha-PLATE"} {
		if !strings.Contains(rec.Body.String(), m) {
			t.Errorf("admin export lacks %q", m)
		}
	}
}

// TestHiddenAreasAreRefused: a role that sees none of the areas gets "not
// allowed" for every route of them, and no area data on the dashboard.
func TestHiddenAreasAreRefused(t *testing.T) {
	e, adm, _, _, _, a, _ := accessUsers(t)
	c := roleUser(t, e, adm, "ProfileOnly", without(auth.PermServiceLogsView, auth.PermRepairsView, auth.PermRemindersView, auth.PermReportsView))
	for _, p := range e.srv.patterns {
		acc := e.srv.routeAccess[p]
		if acc.see == "" || acc.see == auth.PermAttachmentsView {
			continue
		}
		method, path, _ := strings.Cut(p, " ")
		path = strings.ReplaceAll(strings.TrimPrefix(path, "/api/v1"), "{id}", itoa(a.vehicle))
		if rec := c.do(method, path, map[string]any{}); rec.Code != http.StatusForbidden {
			t.Errorf("%s: %d, want 403", p, rec.Code)
		}
	}
	var dash struct {
		Attention   []reminderJSON      `json:"attention"`
		OpenRepairs []repairSummaryJSON `json:"open_repairs"`
		Vehicles    []vehicleJSON       `json:"vehicles"`
	}
	c.call(http.MethodGet, "/dashboard", nil, http.StatusOK, &dash)
	if len(dash.Attention) != 0 || len(dash.OpenRepairs) != 0 {
		t.Errorf("dashboard shows hidden areas: %+v", dash)
	}
	for _, v := range dash.Vehicles {
		if v.ReminderCounts != (reminderCounts{}) || slices.Contains(v.Permissions, auth.PermServiceLogsWrite) {
			t.Errorf("vehicle %d: counts %+v, permissions %v", v.ID, v.ReminderCounts, v.Permissions)
		}
	}
	// The export leaves the areas out.
	rec := c.do(http.MethodGet, "/vehicles/"+itoa(a.vehicle)+"/history?format=json&download=0", nil)
	if strings.Contains(rec.Body.String(), "Alpha brand") || strings.Contains(rec.Body.String(), "Alpha project") {
		t.Errorf("export shows hidden areas: %s", rec.Body)
	}
}

// TestSavingKeepsHiddenFields: a user who can't see a field can't change it
// by saving (SC-003).
func TestSavingKeepsHiddenFields(t *testing.T) {
	e, adm, _, _, _, a, _ := accessUsers(t)
	c := roleUser(t, e, adm, "Helper", without(auth.PermCostsView, auth.PermNotesView, auth.PermVehicleIDsView))

	var log struct {
		ServiceLog serviceLogJSON `json:"service_log"`
	}
	adm.call(http.MethodGet, "/service-logs/"+itoa(a.log), nil, http.StatusOK, &log)
	item := log.ServiceLog.Items[0]
	var cats struct {
		Items []categoryJSON `json:"items"`
	}
	adm.call(http.MethodGet, "/service-categories", nil, http.StatusOK, &cats)
	other := cats.Items[1].ID
	c.call(http.MethodPut, "/service-logs/"+itoa(a.log), map[string]any{
		"vehicle_id": a.vehicle, "service_date": "2026-10-01", "odometer_km": 1000, "notes": "overwritten",
		"items": []map[string]any{
			{"id": item.ID, "category_id": item.CategoryID, "brand": "Changed brand", "cost_cents": 1},
			{"category_id": other, "brand": "New item", "cost_cents": 2},
		},
	}, http.StatusOK, nil)
	adm.call(http.MethodGet, "/service-logs/"+itoa(a.log), nil, http.StatusOK, &log)
	l := log.ServiceLog
	if l.Notes != "Alpha notes" || len(l.Items) != 2 || l.Items[0].Brand != "Changed brand" || l.Items[0].CostCents == nil || *l.Items[0].CostCents != 43219 || l.Items[1].CostCents != nil {
		t.Errorf("service entry after a save without costs and notes: %+v", l)
	}

	c.call(http.MethodPatch, "/repair-parts/"+itoa(a.part), map[string]any{"unit_cost_cents": 1, "name": "Renamed"}, http.StatusOK, nil)
	c.call(http.MethodPatch, "/repairs/"+itoa(a.repair), map[string]any{"procedure_notes": "overwritten"}, http.StatusOK, nil)
	var rep repairResp
	adm.call(http.MethodGet, "/repairs/"+itoa(a.repair), nil, http.StatusOK, &rep)
	if rep.Repair.ProcedureNotes != "Alpha procedure" || rep.Repair.Parts[0].Name != "Renamed" || *rep.Repair.Parts[0].UnitCostCents != 77713 {
		t.Errorf("project after a save without costs and notes: %+v", rep.Repair)
	}

	c.call(http.MethodPatch, "/vehicles/"+itoa(a.vehicle), map[string]any{"license_plate": "X-1", "chassis_code": "B9"}, http.StatusOK, nil)
	var v vehicleResp
	adm.call(http.MethodGet, "/vehicles/"+itoa(a.vehicle), nil, http.StatusOK, &v)
	if v.Vehicle.LicensePlate == nil || *v.Vehicle.LicensePlate != "Alpha-PLATE" || v.Vehicle.ChassisCode != "B9" {
		t.Errorf("vehicle after a save without VIN and plate: %+v", v.Vehicle)
	}

	for _, body := range []map[string]any{{"include_costs": true}, {"include_vin": true}} {
		if code := c.errorCode(http.MethodPut, "/vehicles/"+itoa(a.vehicle)+"/share", body, http.StatusForbidden); code != "auth.forbidden" {
			t.Errorf("share %v: %q", body, code)
		}
	}
}
