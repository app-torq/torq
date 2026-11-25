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
	return a.AllVehicles || a.Vehicles[vehicleID] == LevelEdit
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
	for _, r := range rows {
		a.Vehicles[r.VehicleID] = Level(r.Level)
	}
	return a, nil
}
