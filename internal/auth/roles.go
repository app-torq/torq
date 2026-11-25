package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/db"
	"github.com/app-torq/torq/internal/store"
)

// Built-in role keys.
const (
	builtinAdmin  = "admin"
	builtinMember = "member"
	builtinViewer = "viewer"
)

// builtinNames are the translated names of the built-in roles, used for the
// API's name field and so a custom role can't take a built-in's name.
var builtinNames = map[string]map[string]string{
	builtinAdmin:  {"en": "Administrator", "de": "Administrator"},
	builtinMember: {"en": "Member", "de": "Mitglied"},
	builtinViewer: {"en": "Viewer", "de": "Betrachter"},
}

// RoleInfo is a role with its permissions and number of users.
type RoleInfo struct {
	store.Role
	Permissions []Permission
	UserCount   int64
}

// IsAdmin reports whether this is the Administrator role.
func (r RoleInfo) IsAdmin() bool { return r.BuiltinKey.String == builtinAdmin }

// RoleName returns a role's custom name or the translated built-in name.
func RoleName(r store.Role, locale string) string {
	if r.CustomName.Valid {
		return r.CustomName.String
	}
	names := builtinNames[r.BuiltinKey.String]
	if n, ok := names[locale]; ok {
		return n
	}
	return names["en"]
}

// RoleInput creates or edits a role. On update nil fields stay unchanged; an
// empty Name resets a built-in to its translated name.
type RoleInput struct {
	Name        *string
	Description *string
	AllVehicles *bool
	Permissions *[]string
}

// Roles manages roles and vehicle assignments.
type Roles struct {
	DB    *db.DB
	Clock clock.Clock
}

func getRole(ctx context.Context, q *store.Queries, id int64) (store.Role, error) {
	r, err := q.GetRole(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return store.Role{}, apperr.NotFound()
	}
	return r, err
}

// List returns every role: built-ins first, then custom roles by name.
func (s *Roles) List(ctx context.Context) ([]RoleInfo, error) {
	q := store.New(s.DB.Reader)
	rows, err := q.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	perms, err := q.ListAllRolePermissions(ctx)
	if err != nil {
		return nil, err
	}
	byRole := map[int64]map[string]bool{}
	for _, p := range perms {
		if byRole[p.RoleID] == nil {
			byRole[p.RoleID] = map[string]bool{}
		}
		byRole[p.RoleID][p.Permission] = true
	}
	out := make([]RoleInfo, 0, len(rows))
	for _, r := range rows {
		role := store.Role{
			ID: r.ID, BuiltinKey: r.BuiltinKey, CustomName: r.CustomName, Description: r.Description,
			AllVehicles: r.AllVehicles, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		out = append(out, RoleInfo{Role: role, Permissions: rolePermissions(role, byRole[r.ID]), UserCount: r.UserCount})
	}
	return out, nil
}

// Get returns one role.
func (s *Roles) Get(ctx context.Context, id int64) (RoleInfo, error) {
	return loadRole(ctx, store.New(s.DB.Reader), id)
}

func loadRole(ctx context.Context, q *store.Queries, id int64) (RoleInfo, error) {
	r, err := getRole(ctx, q, id)
	if err != nil {
		return RoleInfo{}, err
	}
	keys, err := q.ListRolePermissions(ctx, id)
	if err != nil {
		return RoleInfo{}, err
	}
	set := map[string]bool{}
	for _, k := range keys {
		set[k] = true
	}
	n, err := q.CountRoleUsers(ctx, sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		return RoleInfo{}, err
	}
	return RoleInfo{Role: r, Permissions: rolePermissions(r, set), UserCount: n}, nil
}

// rolePermissions returns the known permissions in catalog order; the
// Administrator role holds all of them.
func rolePermissions(r store.Role, set map[string]bool) []Permission {
	if r.BuiltinKey.String == builtinAdmin {
		return AllPermissions()
	}
	out := []Permission{}
	for _, info := range Permissions {
		if set[string(info.Key)] {
			out = append(out, info.Key)
		}
	}
	return out
}

// validateRoleName checks length and that no other role is called the same,
// including the translated names of built-ins that weren't renamed.
func validateRoleName(ctx context.Context, q *store.Queries, id int64, name string, builtin bool, v *apperr.Validation) error {
	n := utf8.RuneCountInString(name)
	if (n == 0 && !builtin) || n > 40 {
		v.Add("name", "validation.length")
		return nil
	}
	if n == 0 {
		return nil
	}
	rows, err := q.ListRoles(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if r.ID == id {
			continue
		}
		taken := r.CustomName.Valid && strings.EqualFold(r.CustomName.String, name)
		if !r.CustomName.Valid {
			for _, translated := range builtinNames[r.BuiltinKey.String] {
				taken = taken || strings.EqualFold(translated, name)
			}
		}
		if taken {
			v.Add("name", "validation.duplicate")
			return nil
		}
	}
	return nil
}

func validateDescription(d *string, v *apperr.Validation) {
	if d != nil {
		*d = strings.TrimSpace(*d)
		v.Check(utf8.RuneCountInString(*d) > 200, "description", "validation.length")
	}
}

func setPermissions(ctx context.Context, q *store.Queries, roleID int64, perms []Permission) error {
	if err := q.DeleteRolePermissions(ctx, roleID); err != nil {
		return err
	}
	for _, p := range perms {
		if err := q.AddRolePermission(ctx, store.AddRolePermissionParams{RoleID: roleID, Permission: string(p)}); err != nil {
			return err
		}
	}
	return nil
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// Create adds a custom role.
func (s *Roles) Create(ctx context.Context, in RoleInput) (RoleInfo, error) {
	var out RoleInfo
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		var v apperr.Validation
		name := ""
		if in.Name != nil {
			name = strings.TrimSpace(*in.Name)
		}
		if err := validateRoleName(ctx, q, 0, name, false, &v); err != nil {
			return err
		}
		validateDescription(in.Description, &v)
		var perms []Permission
		if in.Permissions != nil {
			var err error
			if perms, err = ParsePermissions(*in.Permissions); err != nil {
				return err
			}
		}
		if err := v.Err(); err != nil {
			return err
		}
		desc := ""
		if in.Description != nil {
			desc = *in.Description
		}
		all := in.AllVehicles == nil || *in.AllVehicles
		now := clock.Timestamp(s.Clock.Now())
		r, err := q.CreateRole(ctx, store.CreateRoleParams{
			CustomName: sql.NullString{String: name, Valid: true}, Description: desc, AllVehicles: boolInt(all),
			CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		if err := setPermissions(ctx, q, r.ID, perms); err != nil {
			return err
		}
		out, err = loadRole(ctx, q, r.ID)
		return err
	})
	return out, err
}

// Update edits a role. The Administrator role can't be changed.
func (s *Roles) Update(ctx context.Context, id int64, in RoleInput) (RoleInfo, error) {
	var out RoleInfo
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		cur, err := getRole(ctx, q, id)
		if err != nil {
			return err
		}
		if cur.BuiltinKey.String == builtinAdmin {
			return apperr.Conflict("role.builtin_locked", nil)
		}
		var v apperr.Validation
		name := cur.CustomName
		if in.Name != nil {
			n := strings.TrimSpace(*in.Name)
			if err := validateRoleName(ctx, q, id, n, cur.BuiltinKey.Valid, &v); err != nil {
				return err
			}
			name = sql.NullString{String: n, Valid: n != ""}
		}
		validateDescription(in.Description, &v)
		var perms []Permission
		if in.Permissions != nil {
			if perms, err = ParsePermissions(*in.Permissions); err != nil {
				return err
			}
		}
		if err := v.Err(); err != nil {
			return err
		}
		desc, all := cur.Description, cur.AllVehicles
		if in.Description != nil {
			desc = *in.Description
		}
		if in.AllVehicles != nil {
			all = boolInt(*in.AllVehicles)
		}
		if _, err := q.UpdateRole(ctx, store.UpdateRoleParams{
			CustomName: name, Description: desc, AllVehicles: all, UpdatedAt: clock.Timestamp(s.Clock.Now()), ID: id,
		}); err != nil {
			return err
		}
		if in.Permissions != nil {
			if err := setPermissions(ctx, q, id, perms); err != nil {
				return err
			}
		}
		out, err = loadRole(ctx, q, id)
		return err
	})
	return out, err
}

// Delete removes a custom role; its users move to moveTo, which is required
// when the role has users.
func (s *Roles) Delete(ctx context.Context, id int64, moveTo *int64) error {
	return s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		cur, err := getRole(ctx, q, id)
		if err != nil {
			return err
		}
		if cur.BuiltinKey.Valid {
			return apperr.Conflict("role.builtin_locked", nil)
		}
		n, err := q.CountRoleUsers(ctx, sql.NullInt64{Int64: id, Valid: true})
		if err != nil {
			return err
		}
		if n > 0 {
			if moveTo == nil {
				var v apperr.Validation
				v.Add("move_to", "validation.required")
				return v.Err()
			}
			if *moveTo == id {
				return apperr.Conflict("role.move_to_invalid", nil)
			}
			target, err := q.GetRole(ctx, *moveTo)
			if errors.Is(err, sql.ErrNoRows) {
				return apperr.Conflict("role.move_to_invalid", nil)
			}
			if err != nil {
				return err
			}
			if err := q.MoveRoleUsers(ctx, store.MoveRoleUsersParams{
				ToID: sql.NullInt64{Int64: target.ID, Valid: true}, Role: string(legacyRole(target)),
				UpdatedAt: clock.Timestamp(s.Clock.Now()), FromID: sql.NullInt64{Int64: id, Valid: true},
			}); err != nil {
				return err
			}
		}
		return q.DeleteRole(ctx, id)
	})
}

// legacyRole is the admin/member value older clients read for a role.
func legacyRole(r store.Role) Role {
	if r.BuiltinKey.String == builtinAdmin {
		return RoleAdmin
	}
	return RoleMember
}

// Assignment gives a user access to one vehicle.
type Assignment struct {
	VehicleID int64
	Level     Level
}

// VehicleAccess returns the user's vehicle assignments.
func (s *Roles) VehicleAccess(ctx context.Context, userID int64) ([]Assignment, error) {
	q := store.New(s.DB.Reader)
	if _, err := q.GetUser(ctx, userID); errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound()
	} else if err != nil {
		return nil, err
	}
	rows, err := q.ListUserVehicleAccess(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]Assignment, 0, len(rows))
	for _, r := range rows {
		out = append(out, Assignment{VehicleID: r.VehicleID, Level: Level(r.Level)})
	}
	return out, nil
}

// SetVehicleAccess replaces the user's vehicle assignments.
func (s *Roles) SetVehicleAccess(ctx context.Context, userID int64, items []Assignment) ([]Assignment, error) {
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if _, err := q.GetUser(ctx, userID); errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		} else if err != nil {
			return err
		}
		var v apperr.Validation
		seen := map[int64]bool{}
		for _, it := range items {
			if it.Level != LevelView && it.Level != LevelEdit {
				v.Add("items", "validation.invalid_choice")
				break
			}
			if seen[it.VehicleID] {
				v.Add("items", "validation.duplicate")
				break
			}
			seen[it.VehicleID] = true
			if _, err := q.GetVehicle(ctx, it.VehicleID); errors.Is(err, sql.ErrNoRows) {
				v.Add("items", "validation.invalid_choice")
				break
			} else if err != nil {
				return err
			}
		}
		if err := v.Err(); err != nil {
			return err
		}
		if err := q.DeleteUserVehicleAccess(ctx, userID); err != nil {
			return err
		}
		now := clock.Timestamp(s.Clock.Now())
		for _, it := range items {
			if err := q.AddVehicleAccess(ctx, store.AddVehicleAccessParams{
				UserID: userID, VehicleID: it.VehicleID, Level: string(it.Level), CreatedAt: now,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.VehicleAccess(ctx, userID)
}

// GrantVehicle gives the user edit access to a vehicle they just added,
// inside the transaction that created it.
func GrantVehicle(ctx context.Context, tx *sql.Tx, now string, userID, vehicleID int64) error {
	return store.New(tx).AddVehicleAccess(ctx, store.AddVehicleAccessParams{
		UserID: userID, VehicleID: vehicleID, Level: string(LevelEdit), CreatedAt: now,
	})
}

// VehicleAccessCounts returns how many vehicles each user is assigned.
func (s *Roles) VehicleAccessCounts(ctx context.Context) (map[int64]int64, error) {
	rows, err := store.New(s.DB.Reader).CountVehicleAccessByUser(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]int64, len(rows))
	for _, r := range rows {
		out[r.UserID] = r.VehicleCount
	}
	return out, nil
}

// VehicleUser is an active user who can see a vehicle, and how.
type VehicleUser struct {
	UserID      int64
	Username    string
	DisplayName string
	Role        store.Role
	Via         string // role or assignment
	Level       Level
}

// VehicleUsers lists the active users who can see the vehicle.
func (s *Roles) VehicleUsers(ctx context.Context, vehicleID int64) ([]VehicleUser, error) {
	rows, err := store.New(s.DB.Reader).ListVehicleAccessUsers(ctx, vehicleID)
	if err != nil {
		return nil, err
	}
	out := make([]VehicleUser, 0, len(rows))
	for _, r := range rows {
		out = append(out, VehicleUser{
			UserID: r.ID, Username: r.Username, DisplayName: r.DisplayName,
			Role: store.Role{ID: r.RoleID.Int64, BuiltinKey: r.BuiltinKey, CustomName: r.CustomName},
			Via:  r.Via, Level: Level(r.Level),
		})
	}
	return out, nil
}
