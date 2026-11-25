package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/auth"
)

type roleJSON struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	CustomName  *string           `json:"custom_name"`
	BuiltinKey  *string           `json:"builtin_key"`
	Description string            `json:"description"`
	AllVehicles bool              `json:"all_vehicles"`
	Permissions []auth.Permission `json:"permissions"`
	UserCount   int64             `json:"user_count"`
	RequireMFA  bool              `json:"require_mfa"`
}

func toRoleJSON(r auth.RoleInfo, locale string) roleJSON {
	return roleJSON{
		ID: r.ID, Name: auth.RoleName(r.Role, locale), CustomName: strPtr(r.CustomName), BuiltinKey: strPtr(r.BuiltinKey),
		Description: r.Description, AllVehicles: r.AllVehicles == 1, Permissions: r.Permissions, UserCount: r.UserCount,
		RequireMFA: r.RequireMfa == 1,
	}
}

type permissionJSON struct {
	Key   auth.Permission `json:"key"`
	Scope auth.Scope      `json:"scope"`
}

func (s *Server) listRoles(w http.ResponseWriter, r *http.Request) error {
	roles, err := s.roles.List(r.Context())
	if err != nil {
		return err
	}
	locale := currentUser(r).Prefs.Locale
	items := make([]roleJSON, 0, len(roles))
	for _, role := range roles {
		items = append(items, toRoleJSON(role, locale))
	}
	perms := make([]permissionJSON, 0, len(auth.Permissions))
	for _, p := range auth.Permissions {
		perms = append(perms, permissionJSON{Key: p.Key, Scope: p.Scope})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "permissions": perms})
	return nil
}

// roleInput is the body of POST and PATCH /roles. Name is raw so PATCH can
// tell an omitted name from null, which resets a built-in role's name.
type roleInput struct {
	Name        json.RawMessage `json:"name"`
	Description *string         `json:"description"`
	AllVehicles *bool           `json:"all_vehicles"`
	Permissions *[]string       `json:"permissions"`
	RequireMFA  *bool           `json:"require_mfa"`
}

func (in roleInput) domain() (auth.RoleInput, error) {
	out := auth.RoleInput{Description: in.Description, AllVehicles: in.AllVehicles, Permissions: in.Permissions, RequireMFA: in.RequireMFA}
	switch {
	case in.Name == nil:
	case bytes.Equal(in.Name, []byte("null")):
		empty := ""
		out.Name = &empty
	default:
		var name string
		if err := json.Unmarshal(in.Name, &name); err != nil {
			return out, apperr.BadRequest("request.malformed")
		}
		out.Name = &name
	}
	return out, nil
}

func (s *Server) createRole(w http.ResponseWriter, r *http.Request) error {
	var in roleInput
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	d, err := in.domain()
	if err != nil {
		return err
	}
	role, err := s.roles.Create(r.Context(), d)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]any{"role": toRoleJSON(role, currentUser(r).Prefs.Locale)})
	return nil
}

func (s *Server) updateRole(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in roleInput
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	d, err := in.domain()
	if err != nil {
		return err
	}
	role, err := s.roles.Update(r.Context(), id, d)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"role": toRoleJSON(role, currentUser(r).Prefs.Locale)})
	return nil
}

// deleteRole removes a custom role; ?move_to=<id> moves its users first.
func (s *Server) deleteRole(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var moveTo *int64
	if raw := r.URL.Query().Get("move_to"); raw != "" {
		to, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return apperr.BadRequest("request.malformed")
		}
		moveTo = &to
	}
	if err := s.roles.Delete(r.Context(), id, moveTo); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type assignmentJSON struct {
	VehicleID int64      `json:"vehicle_id"`
	Level     auth.Level `json:"level"`
}

func (s *Server) writeVehicleAccess(w http.ResponseWriter, r *http.Request, userID int64, items []auth.Assignment) error {
	u, err := s.users.Get(r.Context(), userID)
	if err != nil {
		return err
	}
	role, err := s.roles.Get(r.Context(), u.RoleID)
	if err != nil {
		return err
	}
	out := make([]assignmentJSON, 0, len(items))
	for _, it := range items {
		out = append(out, assignmentJSON{VehicleID: it.VehicleID, Level: it.Level})
	}
	all := role.AllVehicles == 1 || role.IsAdmin()
	writeJSON(w, http.StatusOK, map[string]any{"all_vehicles": all, "items": out})
	return nil
}

func (s *Server) getVehicleAccess(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	items, err := s.roles.VehicleAccess(r.Context(), id)
	if err != nil {
		return err
	}
	return s.writeVehicleAccess(w, r, id, items)
}

func (s *Server) putVehicleAccess(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Items []assignmentJSON `json:"items"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	items := make([]auth.Assignment, 0, len(in.Items))
	for _, it := range in.Items {
		items = append(items, auth.Assignment{VehicleID: it.VehicleID, Level: it.Level})
	}
	saved, err := s.roles.SetVehicleAccess(r.Context(), id, items)
	if err != nil {
		return err
	}
	return s.writeVehicleAccess(w, r, id, saved)
}

type vehicleUserJSON struct {
	UserID      int64      `json:"user_id"`
	Username    string     `json:"username"`
	DisplayName string     `json:"display_name"`
	RoleID      int64      `json:"role_id"`
	RoleName    string     `json:"role_name"`
	Via         string     `json:"via"`
	Level       auth.Level `json:"level"`
}

// listVehicleUsers lists who can see a vehicle and how (administrators only).
func (s *Server) listVehicleUsers(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if _, err := s.vehicles.Get(r.Context(), id); err != nil {
		return err
	}
	rows, err := s.roles.VehicleUsers(r.Context(), id)
	if err != nil {
		return err
	}
	locale := currentUser(r).Prefs.Locale
	items := make([]vehicleUserJSON, 0, len(rows))
	for _, u := range rows {
		items = append(items, vehicleUserJSON{
			UserID: u.UserID, Username: u.Username, DisplayName: u.DisplayName,
			RoleID: u.Role.ID, RoleName: auth.RoleName(u.Role, locale), Via: u.Via, Level: u.Level,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
	return nil
}
