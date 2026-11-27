package auth

import (
	"slices"
	"testing"
)

func TestAccess(t *testing.T) {
	admin := Access{Admin: true}
	member := Access{AllVehicles: true, Perms: map[Permission]bool{PermServiceLogsWrite: true, PermServiceLogsView: true, PermVehiclesCreate: true}}
	scoped := Access{Perms: map[Permission]bool{PermServiceLogsWrite: true, PermServiceLogsView: true}, Vehicles: map[int64]Level{1: LevelEdit, 2: LevelView}}

	for _, c := range []struct {
		name string
		got  bool
		want bool
	}{
		{"admin can anything", admin.CanOn(PermVehiclesDelete, 9) && admin.Can(PermCatalogUpdate) && admin.Sees(9), true},
		{"member garage permission", member.Can(PermVehiclesCreate), true},
		{"member missing garage permission", member.Can(PermCategoriesManage), false},
		{"member on any vehicle", member.CanOn(PermServiceLogsWrite, 42), true},
		{"member without the permission", member.CanOn(PermServiceLogsDelete, 42), false},
		{"scoped sees assigned", scoped.Sees(1) && scoped.Sees(2), true},
		{"scoped doesn't see others", scoped.Sees(3), false},
		{"scoped edits with edit level", scoped.CanOn(PermServiceLogsWrite, 1), true},
		{"scoped can't edit with view level", scoped.CanOn(PermServiceLogsWrite, 2), false},
		{"scoped can't edit hidden", scoped.CanOn(PermServiceLogsWrite, 3), false},
		{"empty access", Access{}.Sees(1) || Access{}.Can(PermVehiclesCreate), false},
	} {
		if c.got != c.want {
			t.Errorf("%s: got %v", c.name, c.got)
		}
	}

	if ids, all := scoped.Scope(); all || !slices.Equal(ids, []int64{1, 2}) {
		t.Errorf("scoped scope = %v, %v", ids, all)
	}
	if ids, all := member.Scope(); !all || ids != nil {
		t.Errorf("member scope = %v, %v", ids, all)
	}
	if got := scoped.VehiclePermissions(2); len(got) != 0 {
		t.Errorf("view level permissions = %v", got)
	}
	if got := scoped.VehiclePermissions(1); !slices.Equal(got, []Permission{PermServiceLogsWrite}) {
		t.Errorf("edit level permissions = %v", got)
	}
	if got := admin.Granted(); len(got) != len(Permissions) {
		t.Errorf("admin granted %d permissions", len(got))
	}
}

func TestParsePermissions(t *testing.T) {
	got, err := ParsePermissions([]string{"catalog.update", "vehicles.edit", "vehicles.edit"})
	if err != nil || !slices.Equal(got, []Permission{PermVehiclesEdit, PermCatalogUpdate}) {
		t.Fatalf("got %v, %v", got, err)
	}
	if _, err := ParsePermissions([]string{"users.manage"}); err == nil {
		t.Fatal("accepted an unknown permission")
	}
}

func TestChangeNeedsView(t *testing.T) {
	a := Access{AllVehicles: true, Perms: map[Permission]bool{PermServiceLogsWrite: true, PermRepairsWrite: true, PermRepairsView: true}}
	if a.CanOn(PermServiceLogsWrite, 1) {
		t.Error("changes service entries without seeing them")
	}
	if !a.CanOn(PermRepairsWrite, 1) {
		t.Error("can't change projects it sees")
	}
	if len(ViewPermissions()) != 8 {
		t.Errorf("view permissions %v", ViewPermissions())
	}
}

func TestCanShare(t *testing.T) {
	share := map[Permission]bool{PermVehicleAccessShare: true, PermVehiclesEdit: true}
	scoped := Access{Perms: share, Vehicles: map[int64]Level{1: LevelEdit, 2: LevelView, 3: LevelEdit}, Received: map[int64]bool{3: true}}
	for id, want := range map[int64]bool{1: true, 2: false, 3: false, 4: false} {
		if got := scoped.CanOn(PermVehicleAccessShare, id); got != want {
			t.Errorf("assigned vehicle %d: %v, want %v", id, got, want)
		}
	}
	if (Access{Perms: map[Permission]bool{PermVehiclesEdit: true}, Vehicles: map[int64]Level{1: LevelEdit}}).CanOn(PermVehicleAccessShare, 1) {
		t.Error("shares without the permission")
	}
	if !(Access{AllVehicles: true, Perms: share}).CanOn(PermVehicleAccessShare, 9) {
		t.Error("all-vehicles role with edit can't share")
	}
	exportOnly := Access{AllVehicles: true, Perms: map[Permission]bool{PermVehicleAccessShare: true, PermHistoryExport: true, PermSharesManage: true}}
	if exportOnly.CanOn(PermVehicleAccessShare, 9) {
		t.Error("all-vehicles role without a change permission shares")
	}
	if !(Access{Admin: true}).CanOn(PermVehicleAccessShare, 9) {
		t.Error("admin can't share")
	}
}
