package httpapi

import (
	"net/http"
	"slices"
	"testing"

	"github.com/app-torq/torq/internal/auth"
	"github.com/app-torq/torq/internal/config"
)

type rolesResp struct {
	Items       []roleJSON       `json:"items"`
	Permissions []permissionJSON `json:"permissions"`
}

type meResp struct {
	User userJSON `json:"user"`
}

func builtinRole(t *testing.T, c *client, key string) roleJSON {
	t.Helper()
	var roles rolesResp
	c.call(http.MethodGet, "/roles", nil, http.StatusOK, &roles)
	for _, r := range roles.Items {
		if r.BuiltinKey != nil && *r.BuiltinKey == key {
			return r
		}
	}
	t.Fatalf("no built-in role %s", key)
	return roleJSON{}
}

// TestRolesKeepExistingBehavior: after the update, administrators and members
// can do what they could before, and older clients still use admin/member.
func TestRolesKeepExistingBehavior(t *testing.T) {
	e := newEnv(t)
	adm := e.admin()

	var me meResp
	adm.call(http.MethodGet, "/auth/me", nil, http.StatusOK, &me)
	adminID := me.User.ID
	if me.User.Role != "admin" || me.User.RoleName != "Administrator" || me.User.RoleBuiltinKey == nil || *me.User.RoleBuiltinKey != "admin" ||
		me.User.Permissions == nil || len(*me.User.Permissions) != len(auth.Permissions) || me.User.AllVehicles == nil || !*me.User.AllVehicles {
		t.Fatalf("admin me %+v", me.User)
	}

	// An older client creates a member with role "member".
	var created meResp
	adm.call(http.MethodPost, "/users", map[string]string{"username": "anna", "display_name": "Anna", "password": testPassword, "role": "member"}, http.StatusCreated, &created)
	if created.User.Role != "member" || created.User.RoleName != "Member" {
		t.Fatalf("created %+v", created.User)
	}
	member := e.login("anna", testPassword)
	member.call(http.MethodGet, "/auth/me", nil, http.StatusOK, &me)
	if slices.Contains(*me.User.Permissions, auth.PermCatalogUpdate) || len(*me.User.Permissions) != len(auth.Permissions)-1 {
		t.Fatalf("member permissions %v", *me.User.Permissions)
	}
	v := createVehicle(t, member, 1000)
	if len(v.Permissions) != 10 {
		t.Fatalf("member's vehicle permissions %v", v.Permissions)
	}
	member.call(http.MethodDelete, "/vehicles/"+itoa(v.ID), map[string]any{"confirm_name": "Mercedes-Benz C 220 CDI"}, http.StatusNoContent, nil)

	// role and role_id must agree; an unknown role_id is refused.
	adminRole := builtinRole(t, adm, "admin")
	if code := adm.errorCode(http.MethodPatch, "/users/"+itoa(created.User.ID), map[string]any{"role": "member", "role_id": adminRole.ID}, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("code %q", code)
	}
	if code := adm.errorCode(http.MethodPatch, "/users/"+itoa(created.User.ID), map[string]any{"role_id": 999}, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("code %q", code)
	}

	// Promoting with role_id reports role admin for older clients; then the
	// first admin can step down while another remains.
	adm.call(http.MethodPatch, "/users/"+itoa(created.User.ID), map[string]any{"role_id": adminRole.ID}, http.StatusOK, &created)
	if created.User.Role != "admin" {
		t.Fatalf("promoted %+v", created.User)
	}
	adm.call(http.MethodPatch, "/users/"+itoa(adminID), map[string]any{"role": "member"}, http.StatusOK, nil)
	anna := e.login("anna", testPassword)
	if code := anna.errorCode(http.MethodPatch, "/users/"+itoa(created.User.ID), map[string]any{"role": "member"}, http.StatusConflict); code != "user.last_admin" {
		t.Fatalf("code %q", code)
	}
}

func TestRoleManagement(t *testing.T) {
	e := newEnv(t)
	adm := e.admin()

	var roles rolesResp
	adm.call(http.MethodGet, "/roles", nil, http.StatusOK, &roles)
	if len(roles.Items) != 3 || len(roles.Permissions) != len(auth.Permissions) || roles.Items[0].UserCount != 1 {
		t.Fatalf("roles %+v", roles)
	}

	var helper struct {
		Role roleJSON `json:"role"`
	}
	adm.call(http.MethodPost, "/roles", map[string]any{
		"name": " Helper ", "description": "Adds service entries", "permissions": []string{"service_logs.write", "reminders.write", "service_logs.write", "service_logs.view", "reminders.view"},
	}, http.StatusCreated, &helper)
	if helper.Role.Name != "Helper" || !helper.Role.AllVehicles || !slices.Equal(helper.Role.Permissions, []auth.Permission{auth.PermServiceLogsWrite, auth.PermRemindersWrite, auth.PermServiceLogsView, auth.PermRemindersView}) {
		t.Fatalf("helper %+v", helper.Role)
	}

	for _, c := range []struct {
		body  map[string]any
		field string
		code  string
	}{
		{map[string]any{"name": "helper"}, "name", "validation.duplicate"},
		{map[string]any{"name": "Mitglied"}, "name", "validation.duplicate"}, // a built-in's German name
		{map[string]any{"name": ""}, "name", "validation.length"},
		{map[string]any{"name": "Owner", "permissions": []string{"users.manage"}}, "permissions", "validation.invalid_choice"},
	} {
		var out struct {
			Error struct {
				Fields map[string]string `json:"fields"`
			} `json:"error"`
		}
		adm.call(http.MethodPost, "/roles", c.body, http.StatusUnprocessableEntity, &out)
		if out.Error.Fields[c.field] != c.code {
			t.Errorf("%v: fields %v", c.body, out.Error.Fields)
		}
	}

	// Built-ins: Administrator is locked; Member can be renamed and reset.
	adminRole, member := builtinRole(t, adm, "admin"), builtinRole(t, adm, "member")
	if code := adm.errorCode(http.MethodPatch, "/roles/"+itoa(adminRole.ID), map[string]any{"description": "x"}, http.StatusConflict); code != "role.builtin_locked" {
		t.Fatalf("code %q", code)
	}
	if code := adm.errorCode(http.MethodDelete, "/roles/"+itoa(member.ID), nil, http.StatusConflict); code != "role.builtin_locked" {
		t.Fatalf("code %q", code)
	}
	adm.call(http.MethodPatch, "/roles/"+itoa(member.ID), map[string]any{"name": "Family"}, http.StatusOK, &helper)
	if helper.Role.Name != "Family" || helper.Role.CustomName == nil {
		t.Fatalf("renamed %+v", helper.Role)
	}
	adm.call(http.MethodPatch, "/roles/"+itoa(member.ID), map[string]any{"name": nil}, http.StatusOK, &helper)
	if helper.Role.Name != "Member" || helper.Role.CustomName != nil || len(helper.Role.Permissions) != len(auth.Permissions)-1 {
		t.Fatalf("reset %+v", helper.Role)
	}

	// A user in the Helper role can add service entries but not delete them,
	// and permission changes apply on the next request.
	helperRole := roles.Items[0].ID
	adm.call(http.MethodGet, "/roles", nil, http.StatusOK, &roles)
	for _, r := range roles.Items {
		if r.Name == "Helper" {
			helperRole = r.ID
		}
	}
	v := createVehicle(t, adm, 1000)
	var u meResp
	adm.call(http.MethodPost, "/users", map[string]any{"username": "helper", "display_name": "Helper", "password": testPassword, "role_id": helperRole}, http.StatusCreated, &u)
	if u.User.RoleID != helperRole || u.User.Role != "member" || u.User.RoleBuiltinKey != nil {
		t.Fatalf("helper user %+v", u.User)
	}
	c := e.login("helper", testPassword)
	var log struct {
		ServiceLog serviceLogJSON `json:"service_log"`
	}
	var cats struct {
		Items []categoryJSON `json:"items"`
	}
	c.call(http.MethodGet, "/service-categories", nil, http.StatusOK, &cats)
	c.call(http.MethodPost, "/service-logs", map[string]any{
		"vehicle_id": v.ID, "service_date": "2026-10-01", "odometer_km": 1000, "items": []map[string]any{{"category_id": cats.Items[0].ID}},
	}, http.StatusCreated, &log)
	if code := c.errorCode(http.MethodDelete, "/service-logs/"+itoa(log.ServiceLog.ID), nil, http.StatusForbidden); code != "auth.forbidden" {
		t.Fatalf("code %q", code)
	}
	if code := c.errorCode(http.MethodPost, "/vehicles", map[string]any{"make": "VW", "model": "Golf", "current_odometer_km": 1}, http.StatusForbidden); code != "auth.forbidden" {
		t.Fatalf("code %q", code)
	}
	adm.call(http.MethodPatch, "/roles/"+itoa(helperRole), map[string]any{"permissions": []string{"service_logs.write", "service_logs.delete", "service_logs.view"}}, http.StatusOK, nil)
	c.call(http.MethodDelete, "/service-logs/"+itoa(log.ServiceLog.ID), nil, http.StatusOK, nil)

	// Deleting a role with users needs somewhere to move them.
	if code := adm.errorCode(http.MethodDelete, "/roles/"+itoa(helperRole), nil, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("code %q", code)
	}
	if code := adm.errorCode(http.MethodDelete, "/roles/"+itoa(helperRole)+"?move_to="+itoa(helperRole), nil, http.StatusConflict); code != "role.move_to_invalid" {
		t.Fatalf("code %q", code)
	}
	viewer := builtinRole(t, adm, "viewer")
	adm.call(http.MethodDelete, "/roles/"+itoa(helperRole)+"?move_to="+itoa(viewer.ID), nil, http.StatusNoContent, nil)
	c.call(http.MethodGet, "/auth/me", nil, http.StatusOK, &u)
	if u.User.RoleID != viewer.ID || len(*u.User.Permissions) != len(auth.ViewPermissions()) {
		t.Fatalf("moved user %+v", u.User)
	}
}

func TestVehicleAccessAssignments(t *testing.T) {
	_, adm, scoped, viewOnly, _, a, b := accessUsers(t)

	var vs listResponse[vehicleJSON]
	scoped.call(http.MethodGet, "/vehicles", nil, http.StatusOK, &vs)
	if len(vs.Items) != 1 || vs.Items[0].ID != a.vehicle || len(vs.Items[0].Permissions) != 10 {
		t.Fatalf("scoped vehicles %+v", vs.Items)
	}
	viewOnly.call(http.MethodGet, "/vehicles", nil, http.StatusOK, &vs)
	if len(vs.Items) != 1 || len(vs.Items[0].Permissions) != 0 {
		t.Fatalf("view-only vehicles %+v", vs.Items)
	}
	var me meResp
	scoped.call(http.MethodGet, "/auth/me", nil, http.StatusOK, &me)
	if *me.User.AllVehicles {
		t.Fatal("scoped user reported as seeing all vehicles")
	}

	// Who sees vehicle A: the admin and the viewer through their roles, both
	// customers through assignments.
	var who struct {
		Items []vehicleUserJSON `json:"items"`
	}
	adm.call(http.MethodGet, "/vehicles/"+itoa(a.vehicle)+"/access", nil, http.StatusOK, &who)
	if len(who.Items) != 4 {
		t.Fatalf("access to A %+v", who.Items)
	}
	for _, u := range who.Items {
		want := map[string]string{"marek": "role/edit", "viewer": "role/edit", "scoped": "assignment/edit", "viewonly": "assignment/view"}[u.Username]
		if u.Via+"/"+string(u.Level) != want {
			t.Errorf("%s: %s/%s, want %s", u.Username, u.Via, u.Level, want)
		}
	}
	adm.call(http.MethodGet, "/vehicles/"+itoa(b.vehicle)+"/access", nil, http.StatusOK, &who)
	if len(who.Items) != 2 {
		t.Fatalf("access to B %+v", who.Items)
	}

	// The admin's user list shows the number of assigned vehicles.
	var users struct {
		Items []userJSON `json:"items"`
	}
	adm.call(http.MethodGet, "/users", nil, http.StatusOK, &users)
	var scopedID int64
	for _, u := range users.Items {
		if u.Username == "scoped" {
			scopedID = u.ID
			if u.VehicleCount == nil || *u.VehicleCount != 1 || u.RoleName != "Customer" {
				t.Fatalf("scoped in list %+v", u)
			}
		}
	}

	// Invalid assignments are refused; assigning B takes effect on the next request.
	if code := adm.errorCode(http.MethodPut, "/users/"+itoa(scopedID)+"/vehicle-access", map[string]any{
		"items": []map[string]any{{"vehicle_id": 999, "level": "edit"}},
	}, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("code %q", code)
	}
	if code := adm.errorCode(http.MethodPut, "/users/"+itoa(scopedID)+"/vehicle-access", map[string]any{
		"items": []map[string]any{{"vehicle_id": b.vehicle, "level": "manage"}},
	}, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("code %q", code)
	}
	var access struct {
		AllVehicles bool             `json:"all_vehicles"`
		Items       []assignmentJSON `json:"items"`
	}
	adm.call(http.MethodPut, "/users/"+itoa(scopedID)+"/vehicle-access", map[string]any{
		"items": []map[string]any{{"vehicle_id": b.vehicle, "level": "view"}},
	}, http.StatusOK, &access)
	if access.AllVehicles || len(access.Items) != 1 || access.Items[0].VehicleID != b.vehicle {
		t.Fatalf("access %+v", access)
	}
	scoped.call(http.MethodGet, "/vehicles/"+itoa(b.vehicle), nil, http.StatusOK, nil)
	if code := scoped.errorCode(http.MethodGet, "/vehicles/"+itoa(a.vehicle), nil, http.StatusNotFound); code != "not_found" {
		t.Fatalf("code %q", code)
	}

	// A vehicle the scoped user adds is theirs to edit, but they need the
	// garage-wide permission to add it.
	if code := scoped.errorCode(http.MethodPost, "/vehicles", map[string]any{"make": "Own", "model": "Car", "current_odometer_km": 1}, http.StatusForbidden); code != "auth.forbidden" {
		t.Fatalf("code %q", code)
	}
	var roles rolesResp
	adm.call(http.MethodGet, "/roles", nil, http.StatusOK, &roles)
	for _, r := range roles.Items {
		if r.Name == "Customer" {
			perms := append(slices.Clone(r.Permissions), auth.PermVehiclesCreate)
			adm.call(http.MethodPatch, "/roles/"+itoa(r.ID), map[string]any{"permissions": perms}, http.StatusOK, nil)
		}
	}
	var own vehicleResp
	scoped.call(http.MethodPost, "/vehicles", map[string]any{"make": "Own", "model": "Car", "current_odometer_km": 1}, http.StatusCreated, &own)
	if len(own.Vehicle.Permissions) != 10 {
		t.Fatalf("own vehicle permissions %v", own.Vehicle.Permissions)
	}
	scoped.call(http.MethodPatch, "/vehicles/"+itoa(own.Vehicle.ID), map[string]any{"chassis_code": "X1"}, http.StatusOK, nil)
}

func TestRolesDemoLocked(t *testing.T) {
	cfg, err := config.Load(func(k string) string {
		switch k {
		case "TORQ_DATA_DIR":
			return t.TempDir()
		case "TORQ_DEMO_MODE":
			return "true"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	e := newEnvWithConfig(t, cfg)
	adm := e.admin()
	adm.call(http.MethodGet, "/roles", nil, http.StatusOK, nil)
	if code := adm.errorCode(http.MethodPost, "/roles", map[string]any{"name": "Helper"}, http.StatusForbidden); code != "demo.accounts_locked" {
		t.Fatalf("code %q", code)
	}
	if code := adm.errorCode(http.MethodPut, "/users/1/vehicle-access", map[string]any{"items": []any{}}, http.StatusForbidden); code != "demo.accounts_locked" {
		t.Fatalf("code %q", code)
	}
}
