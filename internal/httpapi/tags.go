package httpapi

import (
	"net/http"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/repairs"
)

type tagJSON struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	ProjectCount int64  `json:"project_count"`
}

func toTagJSON(t repairs.Tag) tagJSON {
	return tagJSON{ID: t.ID, Name: t.Name, ProjectCount: t.ProjectCount}
}

func (s *Server) listTags(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.repairs.ListTags(r.Context())
	if err != nil {
		return err
	}
	items := make([]tagJSON, 0, len(rows))
	for _, t := range rows {
		items = append(items, toTagJSON(t))
	}
	writeJSON(w, http.StatusOK, newList(items, int64(len(items))))
	return nil
}

func (s *Server) renameTag(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Name *string `json:"name"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	if in.Name == nil {
		var v apperr.Validation
		v.Add("name", "validation.required")
		return v.Err()
	}
	if _, err := s.repairs.RenameTag(r.Context(), id, *in.Name); err != nil {
		return err
	}
	// Answer with the project count, like the list.
	rows, err := s.repairs.ListTags(r.Context())
	if err != nil {
		return err
	}
	for _, t := range rows {
		if t.ID == id {
			writeJSON(w, http.StatusOK, map[string]any{"tag": toTagJSON(t)})
			return nil
		}
	}
	return apperr.NotFound()
}

func (s *Server) deleteTag(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if err := s.repairs.DeleteTag(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
