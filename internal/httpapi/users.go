package httpapi

import (
	"net/http"
	"strconv"

	"github.com/marekbeckmann/torq/internal/apperr"
	"github.com/marekbeckmann/torq/internal/auth"
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
	items := make([]userJSON, 0, len(users))
	for _, u := range users {
		items = append(items, toUserJSON(u))
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
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	user, err := s.users.Create(r.Context(), auth.NewUser{
		Username: in.Username, DisplayName: in.DisplayName, Password: in.Password,
		Role: auth.Role(in.Role), Locale: currentUser(r).Prefs.Locale,
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, map[string]userJSON{"user": toUserJSON(user)})
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
		Disabled    *bool   `json:"disabled"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	patch := auth.UserPatch{DisplayName: in.DisplayName, Disabled: in.Disabled}
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
	writeJSON(w, http.StatusOK, map[string]userJSON{"user": toUserJSON(user)})
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
