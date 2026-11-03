package httpapi

import (
	"net/http"

	"github.com/app-torq/torq/internal/auth"
)

// sessionJSON is one of the caller's sign-ins in the Devices list. Browser and
// OS are product names; the client builds a label from them when DeviceName
// is empty (research M4).
type sessionJSON struct {
	ID         int64  `json:"id"`
	DeviceName string `json:"device_name"`
	Browser    string `json:"browser"`
	OS         string `json:"os"`
	CreatedAt  string `json:"created_at"`
	LastSeenAt string `json:"last_seen_at"`
	IP         string `json:"ip"`
	Current    bool   `json:"current"`
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) error {
	list, err := s.sessions.List(r.Context(), currentUser(r).ID)
	if err != nil {
		return err
	}
	current, _ := auth.SessionIDFrom(r.Context())
	items := make([]sessionJSON, len(list))
	for i, si := range list {
		items[i] = sessionJSON{
			ID: si.ID, DeviceName: si.DeviceName, Browser: si.Browser, OS: si.OS,
			CreatedAt: si.CreatedAt, LastSeenAt: si.LastSeenAt, IP: si.IP, Current: si.ID == current,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
	return nil
}

// revokeSession ends one of the caller's sessions; ending the current one
// signs the caller out.
func (s *Server) revokeSession(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if err := s.sessions.RevokeOwn(r.Context(), currentUser(r).ID, id); err != nil {
		return err
	}
	if current, _ := auth.SessionIDFrom(r.Context()); id == current && !viaBearer(r) {
		s.clearSessionCookie(w, r)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// revokeOtherSessions signs the caller out everywhere except here.
func (s *Server) revokeOtherSessions(w http.ResponseWriter, r *http.Request) error {
	current, _ := auth.SessionIDFrom(r.Context())
	if err := s.sessions.RevokeAllExcept(r.Context(), currentUser(r).ID, current); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
