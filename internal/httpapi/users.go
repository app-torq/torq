package httpapi

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/auth"
	"github.com/app-torq/torq/internal/store"
)

// pathID parses the {name} path wildcard as a positive int64.
func pathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.NotFound()
	}
	return id, nil
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) error {
	users, err := s.users.List(r.Context())
	if err != nil {
		return err
	}
	roles, err := s.rolesByID(r.Context())
	if err != nil {
		return err
	}
	counts, err := s.roles.VehicleAccessCounts(r.Context())
	if err != nil {
		return err
	}
	withFactors, err := store.New(s.DB.Reader).ListUsersWithFactors(r.Context())
	if err != nil {
		return err
	}
	locale := currentUser(r).Prefs.Locale
	items := make([]userJSON, 0, len(users))
	for _, u := range users {
		out := toUserJSON(u, roles[u.RoleID], locale)
		out.MFAEnabled = slices.Contains(withFactors, u.ID)
		n := counts[u.ID]
		out.VehicleCount = &n
		items = append(items, out)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
	return nil
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
		Role        string `json:"role"`
		RoleID      int64  `json:"role_id"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	user, err := s.users.Create(r.Context(), auth.NewUser{
		Username: in.Username, DisplayName: in.DisplayName, Password: in.Password,
		Role: auth.Role(in.Role), RoleID: in.RoleID, Locale: currentUser(r).Prefs.Locale,
	})
	if err != nil {
		return err
	}
	out, err := s.userOut(r, user)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]userJSON{"user": out})
	return nil
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		DisplayName *string `json:"display_name"`
		Role        *string `json:"role"`
		RoleID      *int64  `json:"role_id"`
		Disabled    *bool   `json:"disabled"`
		MFARequired *bool   `json:"mfa_required"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	patch := auth.UserPatch{DisplayName: in.DisplayName, RoleID: in.RoleID, Disabled: in.Disabled, MFARequired: in.MFARequired}
	if in.Role != nil {
		role := auth.Role(*in.Role)
		patch.Role = &role
	}
	if in.Disabled != nil && *in.Disabled && id == currentUser(r).ID {
		return apperr.Conflict("user.cannot_disable_self", nil)
	}
	user, err := s.users.Update(r.Context(), id, patch)
	if err != nil {
		return err
	}
	out, err := s.userOut(r, user)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]userJSON{"user": out})
	return nil
}

func (s *Server) resetUserPassword(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	if err := s.users.ResetPassword(r.Context(), id, in.NewPassword); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if err := s.users.Delete(r.Context(), currentUser(r).ID, id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
