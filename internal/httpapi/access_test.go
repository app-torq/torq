package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/app-torq/torq/internal/auth"
)

// garageFixture is one vehicle with a record of every kind.
type garageFixture struct {
	vehicle, meter, entry, entryFile, log, logFile, reminder, repair, part, repairFile int64
}

// buildVehicle creates a vehicle whose records all carry name, so tests can
// look for it in responses.
func buildVehicle(t *testing.T, c *client, name string, category int64) garageFixture {
	t.Helper()
	var f garageFixture
	var v vehicleResp
	c.call(http.MethodPost, "/vehicles", map[string]any{"make": name, "model": name + " model", "current_odometer_km": 1000}, http.StatusCreated, &v)
	f.vehicle = v.Vehicle.ID
	base := "/vehicles/" + itoa(f.vehicle)

	var meter struct {
		HourMeter hourMeterJSON `json:"hour_meter"`
	}
	c.call(http.MethodPost, base+"/hour-meters", map[string]any{"name": name + " meter", "hours": 10}, http.StatusCreated, &meter)
	f.meter = meter.HourMeter.ID

	var entry struct {
		Entry specEntryJSON `json:"entry"`
	}
	c.call(http.MethodPost, base+"/spec-entries", map[string]any{"label": name + " label", "value": "1"}, http.StatusCreated, &entry)
	f.entry = entry.Entry.ID

	var log struct {
		ServiceLog serviceLogJSON `json:"service_log"`
	}
	c.call(http.MethodPost, "/service-logs", map[string]any{
		"vehicle_id": f.vehicle, "service_date": "2026-10-01", "odometer_km": 1000, "notes": name + " notes",
		"items": []map[string]any{{"category_id": category, "brand": name + " brand", "cost_cents": 1000}},
	}, http.StatusCreated, &log)
	f.log = log.ServiceLog.ID

	var rem struct {
		Reminder reminderJSON `json:"reminder"`
	}
	c.call(http.MethodPost, "/reminders", map[string]any{"vehicle_id": f.vehicle, "service_category_id": category, "interval_km": 500}, http.StatusCreated, &rem)
	f.reminder = rem.Reminder.ID

	var rep repairResp
	c.call(http.MethodPost, "/repairs", map[string]any{"vehicle_id": f.vehicle, "title": name + " project", "tags": []string{name + "-tag"}}, http.StatusCreated, &rep)
	f.repair = rep.Repair.ID
	var part struct {
		Part partJSON `json:"part"`
	}
	c.call(http.MethodPost, "/repairs/"+itoa(f.repair)+"/parts", map[string]any{"name": name + " part"}, http.StatusCreated, &part)
	f.part = part.Part.ID

	c.call(http.MethodPut, base+"/share", map[string]any{}, http.StatusCreated, nil)

	upload := func(path string) int64 {
		rec := c.upload(path, name+".pdf", testPDF, "")
		if rec.Code != http.StatusCreated {
			t.Fatalf("upload %s: %d %s", path, rec.Code, rec.Body)
		}
		var out struct {
			Attachment attachmentJSON `json:"attachment"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out.Attachment.ID
	}
	f.logFile = upload("/service-logs/" + itoa(f.log) + "/attachments")
	f.repairFile = upload("/repairs/" + itoa(f.repair) + "/attachments")
	f.entryFile = upload("/spec-entries/" + itoa(f.entry) + "/attachments")
	return f
}

// accessRoute is how the route walk calls one route about fixture f.
type accessRoute struct {
	pattern string
	path    func(f garageFixture) string
	body    func(f garageFixture) any
	// list marks member routes whose responses must contain nothing of a
	// hidden vehicle; other member routes are the caller's own account.
	list bool
}

func id(base string, field func(f garageFixture) int64, suffix string) func(f garageFixture) string {
	return func(f garageFixture) string { return base + itoa(field(f)) + suffix }
}

func fixed(path string) func(garageFixture) string { return func(garageFixture) string { return path } }

var (
	vehicleOf  = func(f garageFixture) int64 { return f.vehicle }
	meterOf    = func(f garageFixture) int64 { return f.meter }
	entryOf    = func(f garageFixture) int64 { return f.entry }
	logOf      = func(f garageFixture) int64 { return f.log }
	reminderOf = func(f garageFixture) int64 { return f.reminder }
	repairOf   = func(f garageFixture) int64 { return f.repair }
	partOf     = func(f garageFixture) int64 { return f.part }
	logFileOf  = func(f garageFixture) int64 { return f.logFile }
)

// accessRoutes lists every API route. Routes about a vehicle's data run in
// this order against the editable vehicle, so deletes come last, children
// before their parents.
var accessRoutes = []accessRoute{
	{pattern: "GET /instance"}, {pattern: "GET /instance/logo"}, {pattern: "GET /shared/{token}"},
	{pattern: "GET /setup/status"}, {pattern: "POST /setup"}, {pattern: "POST /auth/login"},
	{pattern: "POST /auth/logout"}, {pattern: "GET /auth/me"}, {pattern: "POST /auth/password"},
	{pattern: "PATCH /me/preferences"}, {pattern: "GET /me/sessions"}, {pattern: "DELETE /me/sessions/{id}"},
	{pattern: "DELETE /me/sessions"},
	{pattern: "POST /auth/mfa"}, {pattern: "POST /auth/mfa/passkey-options"}, {pattern: "POST /auth/passkey/options"},
	{pattern: "POST /auth/passkey"}, {pattern: "GET /me/mfa"}, {pattern: "POST /me/totp"}, {pattern: "DELETE /me/totp"},
	{pattern: "POST /me/totp/confirm"}, {pattern: "POST /me/recovery-codes"}, {pattern: "POST /me/passkeys/options"},
	{pattern: "POST /me/passkeys"}, {pattern: "PATCH /me/passkeys/{id}"}, {pattern: "DELETE /me/passkeys/{id}"},
	{pattern: "DELETE /users/{id}/mfa"},
	{pattern: "GET /users"}, {pattern: "POST /users"}, {pattern: "PATCH /users/{id}"}, {pattern: "POST /users/{id}/password"},
	{pattern: "DELETE /users/{id}"}, {pattern: "GET /users/{id}/vehicle-access"}, {pattern: "PUT /users/{id}/vehicle-access"},
	{pattern: "GET /roles"}, {pattern: "POST /roles"}, {pattern: "PATCH /roles/{id}"}, {pattern: "DELETE /roles/{id}"},
	{pattern: "GET /catalog/kba"}, {pattern: "GET /catalog/makes"}, {pattern: "GET /catalog/models"},
	{pattern: "GET /catalog/generations"}, {pattern: "GET /catalog/engines"}, {pattern: "GET /catalog/meta"},
	{pattern: "POST /catalog/update"},
	{pattern: "GET /vehicles", path: fixed("/vehicles?archived=all"), list: true},
	{pattern: "POST /vehicles"},
	{pattern: "GET /vehicles/{id}", path: id("/vehicles/", vehicleOf, "")},
	{pattern: "PATCH /vehicles/{id}", path: id("/vehicles/", vehicleOf, ""), body: func(garageFixture) any { return map[string]any{"chassis_code": "X"} }},
	{pattern: "POST /vehicles/{id}/archive", path: id("/vehicles/", vehicleOf, "/archive")},
	{pattern: "POST /vehicles/{id}/unarchive", path: id("/vehicles/", vehicleOf, "/unarchive")},
	{pattern: "POST /vehicles/{id}/odometer", path: id("/vehicles/", vehicleOf, "/odometer"), body: func(garageFixture) any { return map[string]any{"odometer_km": 1200} }},
	{pattern: "GET /vehicles/{id}/odometer-readings", path: id("/vehicles/", vehicleOf, "/odometer-readings")},
	{pattern: "POST /vehicles/{id}/hour-meters", path: id("/vehicles/", vehicleOf, "/hour-meters"), body: func(garageFixture) any { return map[string]any{"name": "Second", "hours": 1} }},
	{pattern: "PATCH /hour-meters/{id}", path: id("/hour-meters/", meterOf, ""), body: func(garageFixture) any { return map[string]any{"name": "Renamed"} }},
	{pattern: "POST /hour-meters/{id}/readings", path: id("/hour-meters/", meterOf, "/readings"), body: func(garageFixture) any { return map[string]any{"hours": 12} }},
	{pattern: "GET /hour-meters/{id}/readings", path: id("/hour-meters/", meterOf, "/readings")},
	{pattern: "GET /vehicles/{id}/access", path: id("/vehicles/", vehicleOf, "/access")},
	{pattern: "GET /vehicles/{id}/share", path: id("/vehicles/", vehicleOf, "/share")},
	{pattern: "PUT /vehicles/{id}/share", path: id("/vehicles/", vehicleOf, "/share"), body: func(garageFixture) any { return map[string]any{"include_costs": true} }},
	{pattern: "POST /vehicles/{id}/share/renew", path: id("/vehicles/", vehicleOf, "/share/renew")},
	{pattern: "GET /vehicles/{id}/specs", path: id("/vehicles/", vehicleOf, "/specs")},
	{pattern: "PUT /vehicles/{id}/specs", path: id("/vehicles/", vehicleOf, "/specs"), body: func(garageFixture) any { return map[string]any{"engine_oil_spec": "VW 504 00"} }},
	{pattern: "POST /vehicles/{id}/autofill-specs", path: id("/vehicles/", vehicleOf, "/autofill-specs")},
	{pattern: "POST /vehicles/{id}/spec-entries", path: id("/vehicles/", vehicleOf, "/spec-entries"), body: func(garageFixture) any { return map[string]any{"label": "Wipers", "value": "600 mm"} }},
	{pattern: "PUT /vehicles/{id}/spec-entries/order", path: id("/vehicles/", vehicleOf, "/spec-entries/order"), body: func(f garageFixture) any { return map[string]any{"ids": []int64{f.entry}} }},
	{pattern: "PATCH /spec-entries/{id}", path: id("/spec-entries/", entryOf, ""), body: func(garageFixture) any { return map[string]any{"value": "2"} }},
	{pattern: "GET /service-categories"},
	{pattern: "POST /service-categories"}, {pattern: "PATCH /service-categories/{id}"},
	{pattern: "POST /service-categories/{id}/archive"}, {pattern: "POST /service-categories/{id}/unarchive"},
	{pattern: "DELETE /service-categories/{id}"},
	{pattern: "GET /service-logs", path: fixed("/service-logs"), list: true},
	{pattern: "POST /service-logs", path: fixed("/service-logs"), body: func(f garageFixture) any {
		return map[string]any{"vehicle_id": f.vehicle, "service_date": "2026-10-02", "odometer_km": 1300, "items": []any{}}
	}},
	{pattern: "GET /service-logs/{id}", path: id("/service-logs/", logOf, "")},
	{pattern: "PUT /service-logs/{id}", path: id("/service-logs/", logOf, ""), body: func(f garageFixture) any {
		return map[string]any{"vehicle_id": f.vehicle, "service_date": "2026-10-01", "odometer_km": 1000, "items": []any{}}
	}},
	{pattern: "GET /reminders", path: fixed("/reminders"), list: true},
	{pattern: "POST /reminders", path: fixed("/reminders"), body: func(f garageFixture) any {
		return map[string]any{"vehicle_id": f.vehicle, "title": "Chain", "interval_km": 800}
	}},
	{pattern: "GET /reminders/{id}", path: id("/reminders/", reminderOf, "")},
	{pattern: "PATCH /reminders/{id}", path: id("/reminders/", reminderOf, ""), body: func(f garageFixture) any {
		return map[string]any{"vehicle_id": f.vehicle, "title": "Chain", "interval_km": 900}
	}},
	{pattern: "POST /reminders/{id}/done", path: id("/reminders/", reminderOf, "/done"), body: func(garageFixture) any { return map[string]any{"done_on": "2026-10-03", "odometer_km": 1300} }},
	{pattern: "POST /reminders/{id}/pause", path: id("/reminders/", reminderOf, "/pause")},
	{pattern: "POST /reminders/{id}/resume", path: id("/reminders/", reminderOf, "/resume")},
	{pattern: "GET /dashboard", path: fixed("/dashboard"), list: true},
	{pattern: "GET /repairs", path: fixed("/repairs?status=planned&status=in_progress&status=waiting_on_parts&status=completed"), list: true},
	{pattern: "POST /repairs", path: fixed("/repairs"), body: func(f garageFixture) any { return map[string]any{"vehicle_id": f.vehicle, "title": "Brakes"} }},
	{pattern: "GET /repairs/{id}", path: id("/repairs/", repairOf, "")},
	{pattern: "PATCH /repairs/{id}", path: id("/repairs/", repairOf, ""), body: func(garageFixture) any { return map[string]any{"symptom": "noise"} }},
	{pattern: "POST /repairs/{id}/status", path: id("/repairs/", repairOf, "/status"), body: func(garageFixture) any { return map[string]any{"status": "in_progress"} }},
	{pattern: "POST /repairs/{id}/parts", path: id("/repairs/", repairOf, "/parts"), body: func(garageFixture) any { return map[string]any{"name": "Pads"} }},
	{pattern: "PUT /repairs/{id}/parts/order", path: id("/repairs/", repairOf, "/parts/order"), body: func(f garageFixture) any { return map[string]any{"ids": []int64{f.part}} }},
	{pattern: "PATCH /repair-parts/{id}", path: id("/repair-parts/", partOf, ""), body: func(garageFixture) any { return map[string]any{"status": "ordered"} }},
	{pattern: "GET /repair-priorities", path: fixed("/repair-priorities"), list: true},
	{pattern: "POST /repair-priorities"}, {pattern: "PUT /repair-priorities/order"}, {pattern: "PATCH /repair-priorities/{id}"},
	{pattern: "DELETE /repair-priorities/{id}"},
	{pattern: "GET /repair-tags", path: fixed("/repair-tags"), list: true},
	{pattern: "PATCH /repair-tags/{id}"}, {pattern: "DELETE /repair-tags/{id}"},
	{pattern: "POST /service-logs/{id}/attachments", path: id("/service-logs/", logOf, "/attachments")},
	{pattern: "POST /repairs/{id}/attachments", path: id("/repairs/", repairOf, "/attachments")},
	{pattern: "POST /spec-entries/{id}/attachments", path: id("/spec-entries/", entryOf, "/attachments")},
	{pattern: "PUT /vehicles/{id}/avatar", path: id("/vehicles/", vehicleOf, "/avatar")},
	{pattern: "GET /attachments/{id}", path: id("/attachments/", logFileOf, "")},
	{pattern: "PATCH /attachments/{id}", path: id("/attachments/", logFileOf, ""), body: func(garageFixture) any { return map[string]any{"caption": "receipt"} }},
	{pattern: "GET /webhooks"}, {pattern: "POST /webhooks"}, {pattern: "GET /webhooks/{id}"}, {pattern: "PATCH /webhooks/{id}"},
	{pattern: "DELETE /webhooks/{id}"}, {pattern: "POST /webhooks/{id}/test"}, {pattern: "GET /webhooks/{id}/deliveries"},
	{pattern: "GET /shares"}, {pattern: "GET /settings/instance"}, {pattern: "PATCH /settings/instance"},
	{pattern: "PUT /settings/instance/logo"}, {pattern: "DELETE /settings/instance/logo"},
	{pattern: "GET /reports/costs", path: fixed("/reports/costs?from=2026-01-01&to=2026-12-31"), list: true},
	{pattern: "GET /vehicles/{id}/timeline", path: id("/vehicles/", vehicleOf, "/timeline")},
	{pattern: "GET /vehicles/{id}/history", path: id("/vehicles/", vehicleOf, "/history?format=json&download=0")},
	// Deletes: children first, the vehicle last.
	{pattern: "DELETE /attachments/{id}", path: id("/attachments/", logFileOf, "")},
	{pattern: "DELETE /vehicles/{id}/avatar", path: id("/vehicles/", vehicleOf, "/avatar")},
	{pattern: "DELETE /repair-parts/{id}", path: id("/repair-parts/", partOf, "")},
	{pattern: "DELETE /spec-entries/{id}", path: id("/spec-entries/", entryOf, "")},
	{pattern: "DELETE /hour-meters/{id}", path: id("/hour-meters/", meterOf, "")},
	{pattern: "DELETE /reminders/{id}", path: id("/reminders/", reminderOf, "")},
	{pattern: "DELETE /service-logs/{id}", path: id("/service-logs/", logOf, "")},
	{pattern: "DELETE /repairs/{id}", path: id("/repairs/", repairOf, "")},
	{pattern: "DELETE /vehicles/{id}/share", path: id("/vehicles/", vehicleOf, "/share")},
	{pattern: "DELETE /vehicles/{id}", path: id("/vehicles/", vehicleOf, ""), body: func(garageFixture) any { return map[string]any{"confirm_name": "Alpha Alpha model"} }},
}

// accessUsers sets up a garage with an editable vehicle A and a hidden
// vehicle B and returns the admin plus users in a "Customer" role with every
// per-vehicle permission and no garage permission: scoped edits A, viewOnly
// only sees A; viewer has the built-in Viewer role.
func accessUsers(t *testing.T) (e *env, adm, scoped, viewOnly, viewer *client, a, b garageFixture) {
	e = newEnv(t)
	adm = e.admin()
	var cats struct {
		Items []categoryJSON `json:"items"`
	}
	adm.call(http.MethodGet, "/service-categories", nil, http.StatusOK, &cats)
	a = buildVehicle(t, adm, "Alpha", cats.Items[0].ID)
	b = buildVehicle(t, adm, "Bravo", cats.Items[0].ID)

	vehiclePerms := []string{}
	for _, p := range auth.Permissions {
		if p.Scope == auth.ScopeVehicle {
			vehiclePerms = append(vehiclePerms, string(p.Key))
		}
	}
	var role struct {
		Role roleJSON `json:"role"`
	}
	adm.call(http.MethodPost, "/roles", map[string]any{"name": "Customer", "all_vehicles": false, "permissions": vehiclePerms}, http.StatusCreated, &role)
	var roles struct {
		Items []roleJSON `json:"items"`
	}
	adm.call(http.MethodGet, "/roles", nil, http.StatusOK, &roles)
	var viewerRole int64
	for _, r := range roles.Items {
		if r.BuiltinKey != nil && *r.BuiltinKey == "viewer" {
			viewerRole = r.ID
		}
	}
	newUser := func(name string, roleID int64, level string) *client {
		var u struct {
			User userJSON `json:"user"`
		}
		adm.call(http.MethodPost, "/users", map[string]any{"username": name, "display_name": name, "password": testPassword, "role_id": roleID}, http.StatusCreated, &u)
		if level != "" {
			adm.call(http.MethodPut, "/users/"+itoa(u.User.ID)+"/vehicle-access", map[string]any{
				"items": []map[string]any{{"vehicle_id": a.vehicle, "level": level}},
			}, http.StatusOK, nil)
		}
		return e.login(name, testPassword)
	}
	scoped = newUser("scoped", role.Role.ID, "edit")
	viewOnly = newUser("viewonly", role.Role.ID, "view")
	viewer = newUser("viewer", viewerRole, "")
	return e, adm, scoped, viewOnly, viewer, a, b
}

// TestEveryRouteChecksAccess walks every registered route (SC-002, SC-003):
// administrator routes and garage-wide permissions refuse users without
// them, hidden vehicles answer not found, read-only access refuses changes,
// lists leave out hidden vehicles, and the editable vehicle passes.
func TestEveryRouteChecksAccess(t *testing.T) {
	e, _, scoped, viewOnly, viewer, a, b := accessUsers(t)

	routes := map[string]accessRoute{}
	for _, r := range accessRoutes {
		routes[r.pattern] = r
	}
	for _, p := range e.srv.patterns {
		if _, ok := routes[strings.Replace(p, " /api/v1", " ", 1)]; !ok {
			t.Errorf("%s is missing from accessRoutes: decide how the route walk calls it", p)
		}
	}

	request := func(r accessRoute, f garageFixture) (method, path string, body any) {
		method, _, _ = strings.Cut(r.pattern, " ")
		if r.body != nil {
			body = r.body(f)
		}
		return method, r.path(f), body
	}
	for _, r := range accessRoutes {
		acc := e.srv.routeAccess[strings.Replace(r.pattern, " ", " /api/v1", 1)]
		method, _, _ := strings.Cut(r.pattern, " ")
		switch {
		case acc.level == levelPublic:
		case acc.level == levelAdmin || (acc.perm != "" && !acc.vehicle):
			// Refused before the handler runs, so any path does.
			path := strings.ReplaceAll(strings.SplitN(r.pattern, " ", 2)[1], "{id}", "1")
			for name, c := range map[string]*client{"scoped": scoped, "viewer": viewer} {
				if rec := c.do(method, path, map[string]any{}); rec.Code != http.StatusForbidden {
					t.Errorf("%s as %s: %d, want 403", r.pattern, name, rec.Code)
				}
			}
		case acc.vehicle:
			m, path, body := request(r, b)
			if rec := scoped.do(m, path, body); rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "Bravo") {
				t.Errorf("%s on a hidden vehicle: %d %s, want 404", r.pattern, rec.Code, rec.Body)
			}
			m, path, body = request(r, a)
			for name, c := range map[string]*client{"viewOnly": viewOnly, "viewer": viewer} {
				rec := c.do(m, path, body)
				switch {
				case acc.perm != "" && rec.Code != http.StatusForbidden:
					t.Errorf("%s as %s: %d, want 403", r.pattern, name, rec.Code)
				case acc.perm == "" && m == http.MethodGet && rec.Code != http.StatusOK:
					t.Errorf("%s as %s: %d, want 200; %s", r.pattern, name, rec.Code, rec.Body)
				}
			}
		case r.list:
			for name, c := range map[string]*client{"scoped": scoped, "viewOnly": viewOnly} {
				rec := c.do(method, r.path(a), nil)
				if rec.Code != http.StatusOK || strings.Contains(strings.ToLower(rec.Body.String()), "bravo") {
					t.Errorf("%s as %s shows the hidden vehicle: %d %s", r.pattern, name, rec.Code, rec.Body)
				}
			}
		}
	}

	// The editable vehicle passes every check, in table order.
	for _, r := range accessRoutes {
		acc := e.srv.routeAccess[strings.Replace(r.pattern, " ", " /api/v1", 1)]
		if !acc.vehicle {
			continue
		}
		m, path, body := request(r, a)
		if rec := scoped.do(m, path, body); rec.Code == http.StatusForbidden || rec.Code == http.StatusNotFound {
			if r.pattern == "POST /vehicles/{id}/autofill-specs" && strings.Contains(rec.Body.String(), "specs.autofill_disabled") {
				continue
			}
			t.Errorf("%s on the editable vehicle: %d %s", r.pattern, rec.Code, rec.Body)
		}
	}
}

// TestListsCoverVisibleVehicles checks the lists whose items carry no names:
// reminders, priority counts and the cost report count only vehicle A.
func TestListsCoverVisibleVehicles(t *testing.T) {
	_, adm, scoped, _, _, a, _ := accessUsers(t)

	var rems listResponse[reminderJSON]
	scoped.call(http.MethodGet, "/reminders", nil, http.StatusOK, &rems)
	if len(rems.Items) != 1 || rems.Items[0].VehicleID != a.vehicle {
		t.Fatalf("reminders %+v", rems.Items)
	}

	count := func(c *client) (n int64) {
		var prios struct {
			Items []priorityJSON `json:"items"`
		}
		c.call(http.MethodGet, "/repair-priorities", nil, http.StatusOK, &prios)
		for _, p := range prios.Items {
			n += p.ProjectCount
		}
		return n
	}
	if count(adm) != 2 || count(scoped) != 1 {
		t.Fatalf("priority counts: admin %d, scoped %d", count(adm), count(scoped))
	}

	var costs struct {
		TotalCents int64 `json:"total_cents"`
		ByVehicle  []struct {
			VehicleID int64 `json:"vehicle_id"`
		} `json:"by_vehicle"`
	}
	scoped.call(http.MethodGet, "/reports/costs?from=2026-01-01&to=2026-12-31", nil, http.StatusOK, &costs)
	if costs.TotalCents != 1000 || len(costs.ByVehicle) != 1 || costs.ByVehicle[0].VehicleID != a.vehicle {
		t.Fatalf("costs %+v", costs)
	}
	adm.call(http.MethodGet, "/reports/costs?from=2026-01-01&to=2026-12-31", nil, http.StatusOK, &costs)
	if costs.TotalCents != 2000 {
		t.Fatalf("admin costs %+v", costs)
	}
}
