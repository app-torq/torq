package auth

import (
	"slices"

	"github.com/app-torq/torq/internal/apperr"
)

// Permission is something a role may allow. Keys are stable API values.
type Permission string

// Per-vehicle permissions apply to the vehicles a user can edit.
const (
	PermVehiclesEdit      Permission = "vehicles.edit"
	PermVehiclesDelete    Permission = "vehicles.delete"
	PermSpecsEdit         Permission = "specs.edit"
	PermServiceLogsWrite  Permission = "service_logs.write"
	PermServiceLogsDelete Permission = "service_logs.delete"
	PermRemindersWrite    Permission = "reminders.write"
	PermRepairsWrite      Permission = "repairs.write"
	PermRepairsDelete     Permission = "repairs.delete"
	PermSharesManage      Permission = "shares.manage"
	PermHistoryExport     Permission = "history.export"
	PermVehiclesCreate    Permission = "vehicles.create"
	PermCategoriesManage  Permission = "categories.manage"
	PermRepairListsManage Permission = "repair_lists.manage"
	PermCatalogUpdate     Permission = "catalog.update"
)

// Scope says whether a permission applies per vehicle or to the whole garage.
type Scope string

const (
	ScopeVehicle Scope = "vehicle"
	ScopeGarage  Scope = "garage"
)

// PermissionInfo describes one permission for role editors.
type PermissionInfo struct {
	Key   Permission
	Scope Scope
}

// Permissions lists every permission in the order editors show them.
var Permissions = []PermissionInfo{
	{PermVehiclesEdit, ScopeVehicle},
	{PermVehiclesDelete, ScopeVehicle},
	{PermSpecsEdit, ScopeVehicle},
	{PermServiceLogsWrite, ScopeVehicle},
	{PermServiceLogsDelete, ScopeVehicle},
	{PermRemindersWrite, ScopeVehicle},
	{PermRepairsWrite, ScopeVehicle},
	{PermRepairsDelete, ScopeVehicle},
	{PermSharesManage, ScopeVehicle},
	{PermHistoryExport, ScopeVehicle},
	{PermVehiclesCreate, ScopeGarage},
	{PermCategoriesManage, ScopeGarage},
	{PermRepairListsManage, ScopeGarage},
	{PermCatalogUpdate, ScopeGarage},
}

// ScopeOf returns the permission's scope; ok is false for unknown keys.
func ScopeOf(p Permission) (Scope, bool) {
	for _, info := range Permissions {
		if info.Key == p {
			return info.Scope, true
		}
	}
	return "", false
}

// ParsePermissions validates keys sent by a client and returns them in
// catalog order without duplicates.
func ParsePermissions(keys []string) ([]Permission, error) {
	out := []Permission{}
	for _, k := range keys {
		if _, ok := ScopeOf(Permission(k)); !ok {
			var v apperr.Validation
			v.Add("permissions", "validation.invalid_choice")
			return nil, v.Err()
		}
	}
	for _, info := range Permissions {
		if slices.Contains(keys, string(info.Key)) {
			out = append(out, info.Key)
		}
	}
	return out, nil
}

// AllPermissions returns every key in catalog order.
func AllPermissions() []Permission {
	out := make([]Permission, 0, len(Permissions))
	for _, info := range Permissions {
		out = append(out, info.Key)
	}
	return out
}
