package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/repairs"
	"github.com/app-torq/torq/internal/store"
)

// optionalID is a JSON id that tells "absent" from null: Set is false when
// the field was omitted, Value is nil for null.
type optionalID struct {
	Set   bool
	Value *int64
}

func (o *optionalID) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(b, []byte("null")) {
		o.Value = nil
		return nil
	}
	var v int64
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}

type priorityJSON struct {
	ID           int64   `json:"id"`
	BuiltinKey   *string `json:"builtin_key"`
	CustomName   *string `json:"custom_name"`
	Color        string  `json:"color"`
	SortOrder    int64   `json:"sort_order"`
	IsDefault    bool    `json:"is_default"`
	ProjectCount int64   `json:"project_count"`
}

func toPriorityJSON(p store.RepairPriority, count int64) priorityJSON {
	return priorityJSON{
		ID: p.ID, BuiltinKey: strPtr(p.BuiltinKey), CustomName: strPtr(p.CustomName), Color: p.Color,
		SortOrder: p.SortOrder, IsDefault: p.IsDefault == 1, ProjectCount: count,
	}
}

func (s *Server) listPriorities(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.repairs.ListPriorities(r.Context())
	if err != nil {
		return err
	}
	items := make([]priorityJSON, 0, len(rows))
	for _, p := range rows {
		items = append(items, toPriorityJSON(p.RepairPriority, p.ProjectCount))
	}
	writeJSON(w, http.StatusOK, newList(items, int64(len(items))))
	return nil
}

type priorityInput struct {
	Name      *string `json:"name"`
	Color     *string `json:"color"`
	IsDefault *bool   `json:"is_default"`
}

func (in priorityInput) domain() repairs.PriorityInput {
	return repairs.PriorityInput{Name: in.Name, Color: in.Color, Default: in.IsDefault}
}

// writePriority answers with one priority, including its project count.
func (s *Server) writePriority(w http.ResponseWriter, r *http.Request, status int, id int64) error {
	rows, err := s.repairs.ListPriorities(r.Context())
	if err != nil {
		return err
	}
	for _, p := range rows {
		if p.ID == id {
			writeJSON(w, status, map[string]any{"priority": toPriorityJSON(p.RepairPriority, p.ProjectCount)})
			return nil
		}
	}
	return apperr.NotFound()
}

func (s *Server) createPriority(w http.ResponseWriter, r *http.Request) error {
	var in priorityInput
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	p, err := s.repairs.CreatePriority(r.Context(), in.domain())
	if err != nil {
		return err
	}
	return s.writePriority(w, r, http.StatusCreated, p.ID)
}

func (s *Server) updatePriority(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in priorityInput
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	if _, err := s.repairs.UpdatePriority(r.Context(), id, in.domain()); err != nil {
		return err
	}
	return s.writePriority(w, r, http.StatusOK, id)
}

func (s *Server) reorderPriorities(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return err
	}
	if err := s.repairs.ReorderPriorities(r.Context(), in.IDs); err != nil {
		return err
	}
	return s.listPriorities(w, r)
}

// deletePriority removes a priority; ?move_to=<id> moves its projects first.
func (s *Server) deletePriority(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var moveTo *int64
	if raw := r.URL.Query().Get("move_to"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v <= 0 {
			return badQuery()
		}
		moveTo = &v
	}
	if err := s.repairs.DeletePriority(r.Context(), id, moveTo); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
