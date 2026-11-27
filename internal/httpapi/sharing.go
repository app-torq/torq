package httpapi

import (
	"math"
	"net/http"
	"strconv"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/auth"
)

// Users sharing their own vehicles (#50). Listing and removing work on any
// visible vehicle and only touch the caller's own shares, so a user who lost
// the permission can still take back what they gave; adding and raising need
// vehicle_access.share on the vehicle.

type sharedWithJSON struct {
	UserID      int64      `json:"user_id"`
	Username    string     `json:"username"`
	DisplayName string     `json:"display_name"`
	Level       auth.Level `json:"level"`
	CreatedAt   string     `json:"created_at"`
}

func toSharedWithJSON(sh auth.Share) sharedWithJSON {
	return sharedWithJSON{UserID: sh.UserID, Username: sh.Username, DisplayName: sh.DisplayName, Level: sh.Level, CreatedAt: sh.CreatedAt}
}

func (s *Server) getSharedWith(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathVehicle(r)
	if err != nil {
		return err
	}
	if _, err := s.vehicles.Get(r.Context(), id); err != nil {
		return err
	}
	shares, err := s.roles.SharesBy(r.Context(), currentUser(r).ID, id)
	if err != nil {
		return err
	}
	items := make([]sharedWithJSON, 0, len(shares))
	for _, sh := range shares {
		items = append(items, toSharedWithJSON(sh))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
	return nil
}

func (s *Server) putSharedWith(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathVehicle(r)
	if err != nil {
		return err
	}
	if _, err := s.vehicles.Get(r.Context(), id); err != nil {
		return err
	}
	var in struct {
		Username string     `json:"username"`
		Level    auth.Level `json:"level"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	me := currentUser(r)
	key := strconv.FormatInt(me.ID, 10)
	if wait := s.sharingThrottle.RetryAfter(key); wait > 0 {
		return apperr.Throttled(int(math.Ceil(wait.Seconds())))
	}
	sh, err := s.roles.Share(r.Context(), me.ID, id, in.Username, in.Level)
	if err != nil {
		if e, ok := apperr.As(err); ok && e.Code == auth.ErrShareRefused().Code {
			s.sharingThrottle.Fail(key)
		}
		return err
	}
	s.sharingThrottle.Succeed(key)
	writeJSON(w, http.StatusOK, map[string]any{"share": toSharedWithJSON(sh)})
	return nil
}

func (s *Server) deleteSharedWith(w http.ResponseWriter, r *http.Request) error {
	id, err := s.pathVehicle(r)
	if err != nil {
		return err
	}
	if _, err := s.vehicles.Get(r.Context(), id); err != nil {
		return err
	}
	userID, err := pathID(r, "user_id")
	if err != nil {
		return err
	}
	if err := s.roles.Unshare(r.Context(), currentUser(r).ID, id, userID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
