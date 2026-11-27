package auth

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"github.com/app-torq/torq/internal/store"
)

// Level is how a user may use an assigned vehicle.
type Level string

const (
	LevelView Level = "view" // read only
	LevelEdit Level = "edit" // the role's per-vehicle permissions apply
)

// Access is what a user may do: their role's permissions and the vehicles
// they see. Business logic receives only the visible vehicle IDs from it.
type Access struct {
	Admin       bool
	AllVehicles bool
	Perms       map[Permission]bool
	// Vehicles are the assignments; only used when AllVehicles is false.
	Vehicles map[int64]Level
	// Received are the assigned vehicles another user shared with this one;
	// they can't be shared on (#50).
	Received map[int64]bool
	// MFARequired says the role or the user requires a second factor;
	// MFASetupRequired that one is required and none is set up yet, so the
	// user may only set one up.
	MFARequired      bool
	MFASetupRequired bool
}

// Can reports whether the user holds a garage-wide permission.
func (a Access) Can(p Permission) bool { return a.Admin || a.Perms[p] }

// Sees reports whether the user can see the vehicle.
func (a Access) Sees(vehicleID int64) bool {
	if a.Admin || a.AllVehicles {
		return true
	}
	_, ok := a.Vehicles[vehicleID]
	return ok
}

// CanOn reports whether the user holds a per-vehicle permission on the
// vehicle: the role grants it and the vehicle is editable for them.
func (a Access) CanOn(p Permission, vehicleID int64) bool {
	if a.Admin {
		return true
	}
	if !a.Perms[p] {
		return false
	}
	if v, ok := needsView[p]; ok && !a.Perms[v] {
		return false
	}
	if p == PermVehicleAccessShare {
		return a.holdsForSharing(vehicleID)
	}
	return a.AllVehicles || a.Vehicles[vehicleID] == LevelEdit
}

// holdsForSharing reports whether the user holds the vehicle in a way that
// lets them share it, leaving the share permission itself aside: a role that
// sees all vehicles and may change something on them, or an Edit assignment
// that didn't come from another user's share.
func (a Access) holdsForSharing(vehicleID int64) bool {
	if a.Admin {
		return true
	}
	if a.AllVehicles {
		for _, info := range Permissions {
			if info.Scope == ScopeVehicle && changesVehicle(info.Key) && a.CanOn(info.Key, vehicleID) {
				return true
			}
		}
		return false
	}
	return a.Vehicles[vehicleID] == LevelEdit && !a.Received[vehicleID]
}

// changesVehicle reports whether a per-vehicle permission changes the
// vehicle's records (unlike exporting, the public link or sharing).
func changesVehicle(p Permission) bool {
	switch p {
	case PermHistoryExport, PermSharesManage, PermVehicleAccessShare:
		return false
	}
	return true
}

// Scope returns the visible vehicle IDs; all is true when every vehicle is
// visible, and ids is then nil.
func (a Access) Scope() (ids []int64, all bool) {
	if a.Admin || a.AllVehicles {
		return nil, true
	}
	ids = make([]int64, 0, len(a.Vehicles))
	for id := range a.Vehicles {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids, false
}

// VehiclePermissions returns the per-vehicle permissions the user holds on
// the vehicle, in catalog order.
func (a Access) VehiclePermissions(vehicleID int64) []Permission {
	out := []Permission{}
	for _, info := range Permissions {
		if info.Scope == ScopeVehicle && a.CanOn(info.Key, vehicleID) {
			out = append(out, info.Key)
		}
	}
	return out
}

// Granted returns the permissions the role grants (every one for
// administrators), in catalog order.
func (a Access) Granted() []Permission {
	out := []Permission{}
	for _, info := range Permissions {
		if a.Can(info.Key) {
			out = append(out, info.Key)
		}
	}
	return out
}

// LoadAccess reads the user's role permissions and, for roles that use
// assigned vehicles, their assignments. Unknown permission keys (stored by a
// newer version) are ignored.
func LoadAccess(ctx context.Context, q *store.Queries, u User) (Access, error) {
	role, err := q.GetRole(ctx, u.RoleID)
	if errors.Is(err, sql.ErrNoRows) {
		// Every user has a role; without one, fail closed.
		return Access{Perms: map[Permission]bool{}}, nil
	}
	if err != nil {
		return Access{}, err
	}
	a := Access{Admin: role.BuiltinKey.String == builtinAdmin, AllVehicles: role.AllVehicles == 1, Perms: map[Permission]bool{}}
	a.MFARequired = role.RequireMfa == 1 || u.MFARequired
	if a.MFARequired {
		n, err := q.CountFactors(ctx, u.ID)
		if err != nil {
			return Access{}, err
		}
		a.MFASetupRequired = n == 0
	}
	if a.Admin {
		a.AllVehicles = true
		return a, nil
	}
	keys, err := q.ListRolePermissions(ctx, role.ID)
	if err != nil {
		return Access{}, err
	}
	for _, k := range keys {
		if _, ok := ScopeOf(Permission(k)); ok {
			a.Perms[Permission(k)] = true
		}
	}
	if a.AllVehicles {
		return a, nil
	}
	rows, err := q.ListUserVehicleAccess(ctx, u.ID)
	if err != nil {
		return Access{}, err
	}
	a.Vehicles = make(map[int64]Level, len(rows))
	a.Received = map[int64]bool{}
	for _, r := range rows {
		a.Vehicles[r.VehicleID] = Level(r.Level)
		if r.Origin == OriginShared {
			a.Received[r.VehicleID] = true
		}
	}
	return a, nil
}
