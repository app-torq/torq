package httpapi

import (
	"net/http"
	"slices"
	"testing"

	"github.com/app-torq/torq/internal/auth"
)

// sharingGarage is a garage for the sharing tests: vehicle a belongs to olga
// (Customer, may share, Edit by the administrator); jan and kai can receive
// shares; mia sees all vehicles.
type sharingGarage struct {
	e                *env
	adm              *client
	olga, jan, kai   *client
	ids              map[string]int64
	a, b             int64
	customer, family int64
}

type sharedWithResp struct {
	Items []sharedWithJSON `json:"items"`
}

func newSharingGarage(t *testing.T) *sharingGarage {
	t.Helper()
	g := &sharingGarage{e: newEnv(t), ids: map[string]int64{}}
	g.adm = g.e.admin()
	g.a = createVehicle(t, g.adm, 1000).ID
	g.b = createVehicle(t, g.adm, 2000).ID
	view := []string{}
	for _, p := range auth.ViewPermissions() {
		view = append(view, string(p))
	}
	g.customer = g.role(t, "Customer", false, append(slices.Clone(view), "vehicles.edit", "service_logs.write", "vehicle_access.share"))
	g.family = g.role(t, "Family", false, append(slices.Clone(view), "service_logs.write"))
	g.user(t, "olga", g.customer)
	g.user(t, "jan", g.family)
	g.user(t, "kai", g.customer)
	g.assign(t, "olga", g.a, "edit")
	g.olga = g.e.login("olga", testPassword)
	g.jan = g.e.login("jan", testPassword)
	g.kai = g.e.login("kai", testPassword)
	return g
}

func (g *sharingGarage) role(t *testing.T, name string, all bool, perms []string) int64 {
	t.Helper()
	var out struct {
		Role roleJSON `json:"role"`
	}
	g.adm.call(http.MethodPost, "/roles", map[string]any{"name": name, "all_vehicles": all, "permissions": perms}, http.StatusCreated, &out)
	return out.Role.ID
}

func (g *sharingGarage) user(t *testing.T, name string, roleID int64) {
	t.Helper()
	var out meResp
	g.adm.call(http.MethodPost, "/users", map[string]any{"username": name, "display_name": "Name " + name, "password": testPassword, "role_id": roleID}, http.StatusCreated, &out)
	g.ids[name] = out.User.ID
}

func (g *sharingGarage) assign(t *testing.T, name string, vehicleID int64, level string) {
	t.Helper()
	g.adm.call(http.MethodPut, "/users/"+itoa(g.ids[name])+"/vehicle-access", map[string]any{
		"items": []map[string]any{{"vehicle_id": vehicleID, "level": level}},
	}, http.StatusOK, nil)
}

func sharedWith(vehicleID int64) string { return "/vehicles/" + itoa(vehicleID) + "/shared-with" }

func (g *sharingGarage) share(t *testing.T, c *client, vehicleID int64, username, level string) {
	t.Helper()
	c.call(http.MethodPut, sharedWith(vehicleID), map[string]any{"username": username, "level": level}, http.StatusOK, nil)
}

func (g *sharingGarage) janSees(t *testing.T, vehicleID int64) bool {
	t.Helper()
	return g.jan.do(http.MethodGet, "/vehicles/"+itoa(vehicleID), nil).Code == http.StatusOK
}

func TestShareOwnVehicle(t *testing.T) {
	g := newSharingGarage(t)

	var v vehicleResp
	g.olga.call(http.MethodGet, "/vehicles/"+itoa(g.a), nil, http.StatusOK, &v)
	if !slices.Contains(v.Vehicle.Permissions, auth.PermVehicleAccessShare) || v.Vehicle.SharedBy != nil {
		t.Fatalf("olga's vehicle %+v", v.Vehicle)
	}
	var list sharedWithResp
	g.olga.call(http.MethodGet, sharedWith(g.a), nil, http.StatusOK, &list)
	if len(list.Items) != 0 {
		t.Fatalf("shares before %+v", list.Items)
	}

	// View: the vehicle appears for jan, read only, naming olga.
	var put struct {
		Share sharedWithJSON `json:"share"`
	}
	g.olga.call(http.MethodPut, sharedWith(g.a), map[string]any{"username": "JAN", "level": "view"}, http.StatusOK, &put)
	if put.Share.UserID != g.ids["jan"] || put.Share.Username != "jan" || put.Share.Level != auth.LevelView {
		t.Fatalf("share %+v", put.Share)
	}
	var vs listResponse[vehicleJSON]
	g.jan.call(http.MethodGet, "/vehicles", nil, http.StatusOK, &vs)
	if len(vs.Items) != 1 || vs.Items[0].ID != g.a || vs.Items[0].SharedBy == nil || vs.Items[0].SharedBy.DisplayName != "Name olga" || len(vs.Items[0].Permissions) != 0 {
		t.Fatalf("jan's vehicles %+v", vs.Items)
	}
	var cats struct {
		Items []categoryJSON `json:"items"`
	}
	g.adm.call(http.MethodGet, "/service-categories", nil, http.StatusOK, &cats)
	log := map[string]any{"vehicle_id": g.a, "service_date": "2026-01-01", "odometer_km": 1100, "items": []map[string]any{{"category_id": cats.Items[0].ID}}}
	if code := g.jan.errorCode(http.MethodPost, "/service-logs", log, http.StatusForbidden); code != "auth.forbidden" {
		t.Fatalf("view share wrote: %q", code)
	}

	// Edit: jan's own role decides what edit allows.
	g.share(t, g.olga, g.a, "jan", "edit")
	var created struct {
		ServiceLog struct {
			ID int64 `json:"id"`
		} `json:"service_log"`
	}
	g.jan.call(http.MethodPost, "/service-logs", log, http.StatusCreated, &created)
	if code := g.jan.errorCode(http.MethodDelete, "/service-logs/"+itoa(created.ServiceLog.ID), nil, http.StatusForbidden); code != "auth.forbidden" {
		t.Fatalf("family deleted an entry: %q", code)
	}
	g.jan.call(http.MethodGet, "/vehicles/"+itoa(g.a), nil, http.StatusOK, &v)
	if slices.Contains(v.Vehicle.Permissions, auth.PermVehicleAccessShare) || !slices.Contains(v.Vehicle.Permissions, auth.PermServiceLogsWrite) {
		t.Fatalf("jan's permissions %v", v.Vehicle.Permissions)
	}
	g.olga.call(http.MethodGet, sharedWith(g.a), nil, http.StatusOK, &list)
	if len(list.Items) != 1 || list.Items[0].Level != auth.LevelEdit {
		t.Fatalf("shares %+v", list.Items)
	}

	// Taking it back hides the vehicle from jan everywhere.
	g.olga.call(http.MethodDelete, sharedWith(g.a)+"/"+itoa(g.ids["jan"]), nil, http.StatusNoContent, nil)
	if g.janSees(t, g.a) {
		t.Fatal("jan still sees the vehicle")
	}
	g.jan.call(http.MethodGet, "/vehicles", nil, http.StatusOK, &vs)
	if len(vs.Items) != 0 {
		t.Fatalf("jan's vehicles after revoke %+v", vs.Items)
	}
	g.olga.call(http.MethodDelete, sharedWith(g.a)+"/"+itoa(g.ids["jan"]), nil, http.StatusNotFound, nil)

	// Without the permission: no new shares, but existing ones can be taken back.
	g.share(t, g.olga, g.a, "jan", "view")
	g.adm.call(http.MethodPatch, "/roles/"+itoa(g.customer), map[string]any{"permissions": []string{"vehicles.edit", "service_logs.view"}}, http.StatusOK, nil)
	if !g.janSees(t, g.a) {
		t.Fatal("losing the permission ended the share")
	}
	if code := g.olga.errorCode(http.MethodPut, sharedWith(g.a), map[string]any{"username": "kai", "level": "view"}, http.StatusForbidden); code != "auth.forbidden" {
		t.Fatalf("code %q", code)
	}
	g.olga.call(http.MethodGet, sharedWith(g.a), nil, http.StatusOK, &list)
	if len(list.Items) != 1 {
		t.Fatalf("shares without the permission %+v", list.Items)
	}
	g.olga.call(http.MethodDelete, sharedWith(g.a)+"/"+itoa(g.ids["jan"]), nil, http.StatusNoContent, nil)
}

func TestShareRefusalsLookAlike(t *testing.T) {
	g := newSharingGarage(t)
	g.user(t, "mia", builtinRole(t, g.adm, "member").ID)
	g.user(t, "gone", g.family)
	g.adm.call(http.MethodPatch, "/users/"+itoa(g.ids["gone"]), map[string]any{"disabled": true}, http.StatusOK, nil)
	g.user(t, "held", g.family)
	g.assign(t, "held", g.a, "view")

	var first string
	for _, name := range []string{"nobody", "admin", "mia", "olga", "gone", "held"} {
		rec := g.olga.do(http.MethodPut, sharedWith(g.a), map[string]any{"username": name, "level": "view"})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: %d %s", name, rec.Code, rec.Body)
		}
		if first == "" {
			first = rec.Body.String()
		}
		if rec.Body.String() != first {
			t.Errorf("%s answers differently: %s vs %s", name, rec.Body, first)
		}
	}
	if code := g.olga.errorCode(http.MethodPut, sharedWith(g.a), map[string]any{"username": " ", "level": "view"}, http.StatusUnprocessableEntity); code != "validation.failed" {
		t.Fatalf("empty username: %q", code)
	}

	// After ten refusals the sharer has to wait; other users don't.
	for i := 0; i < 4; i++ {
		g.olga.errorCode(http.MethodPut, sharedWith(g.a), map[string]any{"username": "nobody", "level": "view"}, http.StatusUnprocessableEntity)
	}
	rec := g.olga.do(http.MethodPut, sharedWith(g.a), map[string]any{"username": "jan", "level": "view"})
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("after ten refusals: %d %s", rec.Code, rec.Body)
	}
	g.assign(t, "kai", g.b, "edit")
	g.share(t, g.kai, g.b, "jan", "view")
}

func TestShareAdminOverview(t *testing.T) {
	g := newSharingGarage(t)
	g.share(t, g.olga, g.a, "jan", "view")

	var users struct {
		Items []vehicleUserJSON `json:"items"`
	}
	g.adm.call(http.MethodGet, "/vehicles/"+itoa(g.a)+"/access", nil, http.StatusOK, &users)
	origins := map[string]string{}
	for _, u := range users.Items {
		if u.Origin != nil {
			origins[u.Username] = *u.Origin
		}
		if u.Username == "jan" && (u.GrantedBy == nil || u.GrantedBy.Username != "olga") {
			t.Fatalf("jan granted by %+v", u.GrantedBy)
		}
		if u.Via == "role" && (u.Origin != nil || u.GrantedBy != nil) {
			t.Fatalf("role user %+v", u)
		}
	}
	if origins["jan"] != "shared" || origins["olga"] != "admin" {
		t.Fatalf("origins %v", origins)
	}

	type accessResp struct {
		Items []assignmentJSON `json:"items"`
	}
	var acc accessResp
	path := "/users/" + itoa(g.ids["jan"]) + "/vehicle-access"
	g.adm.call(http.MethodGet, path, nil, http.StatusOK, &acc)
	if len(acc.Items) != 1 || acc.Items[0].Origin != "shared" || acc.Items[0].GrantedBy == nil || acc.Items[0].GrantedBy.DisplayName != "Name olga" {
		t.Fatalf("jan's assignments %+v", acc.Items)
	}
	// Saving unchanged keeps the share; changing the level makes it the administrator's.
	g.adm.call(http.MethodPut, path, map[string]any{"items": []map[string]any{{"vehicle_id": g.a, "level": "view"}, {"vehicle_id": g.b, "level": "view"}}}, http.StatusOK, &acc)
	if len(acc.Items) != 2 || acc.Items[0].Origin != "shared" || acc.Items[1].Origin != "admin" || acc.Items[1].GrantedBy != nil {
		t.Fatalf("after unchanged save %+v", acc.Items)
	}
	g.adm.call(http.MethodPut, path, map[string]any{"items": []map[string]any{{"vehicle_id": g.a, "level": "edit"}}}, http.StatusOK, &acc)
	if len(acc.Items) != 1 || acc.Items[0].Origin != "admin" || acc.Items[0].GrantedBy != nil {
		t.Fatalf("after level change %+v", acc.Items)
	}
	var list sharedWithResp
	g.olga.call(http.MethodGet, sharedWith(g.a), nil, http.StatusOK, &list)
	if len(list.Items) != 0 {
		t.Fatalf("olga still lists %+v", list.Items)
	}
	// Olga can't touch the administrator's assignment, nor share over it.
	g.olga.call(http.MethodDelete, sharedWith(g.a)+"/"+itoa(g.ids["jan"]), nil, http.StatusNotFound, nil)
	if code := g.olga.errorCode(http.MethodPut, sharedWith(g.a), map[string]any{"username": "jan", "level": "view"}, http.StatusUnprocessableEntity); code != "sharing.refused" {
		t.Fatalf("code %q", code)
	}
}

func TestSharesEndWithTheSharer(t *testing.T) {
	g := newSharingGarage(t)

	// Losing Edit on the vehicle ends olga's shares.
	g.share(t, g.olga, g.a, "jan", "view")
	g.assign(t, "olga", g.a, "view")
	if g.janSees(t, g.a) {
		t.Fatal("share survived the sharer's downgrade")
	}

	// Disabling keeps them, deleting ends them.
	g.assign(t, "olga", g.a, "edit")
	g.share(t, g.olga, g.a, "jan", "view")
	g.adm.call(http.MethodPatch, "/users/"+itoa(g.ids["olga"]), map[string]any{"disabled": true}, http.StatusOK, nil)
	if !g.janSees(t, g.a) {
		t.Fatal("disabling the sharer ended the share")
	}
	g.adm.call(http.MethodDelete, "/users/"+itoa(g.ids["olga"]), nil, http.StatusNoContent, nil)
	if g.janSees(t, g.a) {
		t.Fatal("share survived the sharer's deletion")
	}

	// A role that sees all vehicles loses its last change permission.
	workshop := g.role(t, "Workshop", true, []string{"vehicles.edit", "vehicle_access.share"})
	g.user(t, "ben", workshop)
	ben := g.e.login("ben", testPassword)
	g.share(t, ben, g.b, "jan", "edit")
	g.adm.call(http.MethodPatch, "/roles/"+itoa(workshop), map[string]any{"permissions": []string{"vehicle_access.share"}}, http.StatusOK, nil)
	if g.janSees(t, g.b) {
		t.Fatal("share survived the sharer's role losing edit")
	}

	// Moving the sharer to a role without the vehicle ends it too.
	g.adm.call(http.MethodPatch, "/roles/"+itoa(workshop), map[string]any{"permissions": []string{"vehicles.edit", "vehicle_access.share"}}, http.StatusOK, nil)
	g.share(t, ben, g.b, "jan", "edit")
	g.adm.call(http.MethodPatch, "/users/"+itoa(g.ids["ben"]), map[string]any{"role_id": g.customer}, http.StatusOK, nil)
	if g.janSees(t, g.b) {
		t.Fatal("share survived the sharer's role change")
	}
}

func TestSharesCannotBePassedOn(t *testing.T) {
	g := newSharingGarage(t)
	g.share(t, g.olga, g.a, "kai", "edit")

	var v vehicleResp
	g.kai.call(http.MethodGet, "/vehicles/"+itoa(g.a), nil, http.StatusOK, &v)
	if slices.Contains(v.Vehicle.Permissions, auth.PermVehicleAccessShare) || !slices.Contains(v.Vehicle.Permissions, auth.PermVehiclesEdit) {
		t.Fatalf("kai's permissions %v", v.Vehicle.Permissions)
	}
	if code := g.kai.errorCode(http.MethodPut, sharedWith(g.a), map[string]any{"username": "jan", "level": "view"}, http.StatusForbidden); code != "auth.forbidden" {
		t.Fatalf("passed on: %q", code)
	}

	// A vehicle kai added himself can be shared.
	perms := []string{"vehicles.edit", "vehicles.create", "vehicle_access.share"}
	g.adm.call(http.MethodPatch, "/roles/"+itoa(g.customer), map[string]any{"permissions": perms}, http.StatusOK, nil)
	own := createVehicle(t, g.kai, 10)
	g.share(t, g.kai, own.ID, "jan", "view")
	if !g.janSees(t, own.ID) {
		t.Fatal("jan doesn't see kai's vehicle")
	}
}

func TestShareFromRolesThatSeeAllVehicles(t *testing.T) {
	g := newSharingGarage(t)
	workshop := g.role(t, "Workshop", true, []string{"vehicles.edit", "vehicle_access.share"})
	lookers := g.role(t, "Lookers", true, []string{"vehicle_access.share"})
	g.user(t, "ben", workshop)
	g.user(t, "eva", workshop)
	g.user(t, "lou", lookers)
	ben, eva, lou := g.e.login("ben", testPassword), g.e.login("eva", testPassword), g.e.login("lou", testPassword)

	g.share(t, ben, g.b, "jan", "view")
	if !g.janSees(t, g.b) {
		t.Fatal("jan doesn't see the workshop's share")
	}
	if code := lou.errorCode(http.MethodPut, sharedWith(g.b), map[string]any{"username": "kai", "level": "view"}, http.StatusForbidden); code != "auth.forbidden" {
		t.Fatalf("view-only role shared: %q", code)
	}
	// Each sharer sees only their own shares; one recipient, one sharer.
	if code := eva.errorCode(http.MethodPut, sharedWith(g.b), map[string]any{"username": "jan", "level": "edit"}, http.StatusUnprocessableEntity); code != "sharing.refused" {
		t.Fatalf("second sharer: %q", code)
	}
	g.share(t, eva, g.b, "kai", "view")
	var list sharedWithResp
	ben.call(http.MethodGet, sharedWith(g.b), nil, http.StatusOK, &list)
	if len(list.Items) != 1 || list.Items[0].Username != "jan" {
		t.Fatalf("ben's shares %+v", list.Items)
	}
	eva.call(http.MethodGet, sharedWith(g.b), nil, http.StatusOK, &list)
	if len(list.Items) != 1 || list.Items[0].Username != "kai" {
		t.Fatalf("eva's shares %+v", list.Items)
	}
}
