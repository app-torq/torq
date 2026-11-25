package repairs

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/db"
	"github.com/app-torq/torq/internal/store"
)

// PriorityColors is the palette priorities can use. The web app maps each key
// to colors that stay readable on the dark theme; the list can grow.
var PriorityColors = []string{"red", "orange", "amber", "yellow", "lime", "green", "teal", "sky", "blue", "violet", "pink", "zinc"}

// Priority is a project priority with the number of projects using it.
type Priority struct {
	store.RepairPriority
	ProjectCount int64
}

// PriorityInput creates or edits a priority. On update, a nil Name keeps the
// name, an empty one resets a built-in to its translated name, and a nil
// Color keeps the color.
type PriorityInput struct {
	Name    *string
	Color   *string
	Default *bool
}

func getPriority(ctx context.Context, q *store.Queries, id int64) (store.RepairPriority, error) {
	p, err := q.GetRepairPriority(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return store.RepairPriority{}, apperr.NotFound()
	}
	return p, err
}

// ListPriorities returns all priorities in order, with project counts over
// the visible vehicles (nil = all).
func (s *Service) ListPriorities(ctx context.Context, visible []int64) ([]Priority, error) {
	rows, err := store.New(s.DB.Reader).ListRepairPriorities(ctx, db.Scope(visible))
	if err != nil {
		return nil, err
	}
	out := make([]Priority, 0, len(rows))
	for _, r := range rows {
		out = append(out, Priority{RepairPriority: store.RepairPriority{
			ID: r.ID, BuiltinKey: r.BuiltinKey, CustomName: r.CustomName, Color: r.Color, SortOrder: r.SortOrder,
			IsDefault: r.IsDefault, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}, ProjectCount: r.ProjectCount})
	}
	return out, nil
}

func validatePriority(in PriorityInput, builtin bool, v *apperr.Validation) {
	if in.Name != nil {
		*in.Name = strings.TrimSpace(*in.Name)
		n := utf8.RuneCountInString(*in.Name)
		v.Check((n == 0 && !builtin) || n > 40, "name", "validation.length")
	}
	v.Check(in.Color != nil && !slices.Contains(PriorityColors, *in.Color), "color", "validation.invalid_choice")
}

func mapPriorityUnique(err error) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		var v apperr.Validation
		v.Add("name", "validation.duplicate")
		return v.Err()
	}
	return err
}

func setDefault(ctx context.Context, q *store.Queries, id int64, on bool, isDefault bool) error {
	switch {
	case on && !isDefault:
		if err := q.ClearDefaultRepairPriority(ctx); err != nil {
			return err
		}
		return q.SetDefaultRepairPriority(ctx, id)
	case !on && isDefault:
		return q.ClearDefaultRepairPriority(ctx)
	}
	return nil
}

// CreatePriority adds a custom priority at the end of the list.
func (s *Service) CreatePriority(ctx context.Context, in PriorityInput) (store.RepairPriority, error) {
	var v apperr.Validation
	v.Check(in.Name == nil, "name", "validation.required")
	v.Check(in.Color == nil, "color", "validation.required")
	validatePriority(in, false, &v)
	if err := v.Err(); err != nil {
		return store.RepairPriority{}, err
	}
	var out store.RepairPriority
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		now := clock.Timestamp(s.Clock.Now())
		p, err := q.CreateRepairPriority(ctx, store.CreateRepairPriorityParams{
			CustomName: sql.NullString{String: *in.Name, Valid: true}, Color: *in.Color, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return mapPriorityUnique(err)
		}
		if in.Default != nil {
			if err := setDefault(ctx, q, p.ID, *in.Default, false); err != nil {
				return err
			}
		}
		out, err = getPriority(ctx, q, p.ID)
		return err
	})
	return out, err
}

// UpdatePriority renames, recolors or (un)sets the default.
func (s *Service) UpdatePriority(ctx context.Context, id int64, in PriorityInput) (store.RepairPriority, error) {
	var out store.RepairPriority
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		cur, err := getPriority(ctx, q, id)
		if err != nil {
			return err
		}
		var v apperr.Validation
		validatePriority(in, cur.BuiltinKey.Valid, &v)
		if err := v.Err(); err != nil {
			return err
		}
		name, color := cur.CustomName, cur.Color
		if in.Name != nil {
			name = sql.NullString{String: *in.Name, Valid: *in.Name != ""}
		}
		if in.Color != nil {
			color = *in.Color
		}
		if _, err := q.UpdateRepairPriority(ctx, store.UpdateRepairPriorityParams{
			CustomName: name, Color: color, UpdatedAt: clock.Timestamp(s.Clock.Now()), ID: id,
		}); err != nil {
			return mapPriorityUnique(err)
		}
		if in.Default != nil {
			if err := setDefault(ctx, q, id, *in.Default, cur.IsDefault == 1); err != nil {
				return err
			}
		}
		out, err = getPriority(ctx, q, id)
		return err
	})
	return out, err
}

// ReorderPriorities sets the order; ids must list every priority once.
func (s *Service) ReorderPriorities(ctx context.Context, ids []int64) error {
	return s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		rows, err := q.ListRepairPriorities(ctx, sql.NullString{})
		if err != nil {
			return err
		}
		known := map[int64]bool{}
		for _, r := range rows {
			known[r.ID] = true
		}
		seen := map[int64]bool{}
		for _, id := range ids {
			if !known[id] || seen[id] {
				return apperr.Unprocessable("priority.order_mismatch", nil)
			}
			seen[id] = true
		}
		if len(seen) != len(known) {
			return apperr.Unprocessable("priority.order_mismatch", nil)
		}
		for i, id := range ids {
			if err := q.SetRepairPriorityOrder(ctx, store.SetRepairPriorityOrderParams{SortOrder: int64(i + 1), ID: id}); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeletePriority removes a priority. Its projects move to moveTo, or lose
// their priority when moveTo is nil.
func (s *Service) DeletePriority(ctx context.Context, id int64, moveTo *int64) error {
	return s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if _, err := getPriority(ctx, q, id); err != nil {
			return err
		}
		if moveTo != nil {
			if *moveTo == id {
				return invalidPriority("move_to")
			}
			if _, err := getPriority(ctx, q, *moveTo); err != nil {
				return invalidPriority("move_to")
			}
			if err := q.MoveRepairsToPriority(ctx, store.MoveRepairsToPriorityParams{
				ToID: sql.NullInt64{Int64: *moveTo, Valid: true}, FromID: sql.NullInt64{Int64: id, Valid: true},
			}); err != nil {
				return err
			}
		}
		return q.DeleteRepairPriority(ctx, id)
	})
}

func invalidPriority(field string) error {
	var v apperr.Validation
	v.Add(field, "validation.invalid_choice")
	return v.Err()
}

// resolvePriority returns the priority a project should get: the default when
// useDefault is set, otherwise id after checking that it exists.
func resolvePriority(ctx context.Context, q *store.Queries, id *int64, useDefault bool) (sql.NullInt64, error) {
	if useDefault {
		p, err := q.GetDefaultRepairPriority(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return sql.NullInt64{}, nil
		}
		if err != nil {
			return sql.NullInt64{}, err
		}
		return sql.NullInt64{Int64: p.ID, Valid: true}, nil
	}
	if id == nil {
		return sql.NullInt64{}, nil
	}
	if _, err := getPriority(ctx, q, *id); err != nil {
		if e, ok := apperr.As(err); ok && e.Code == "not_found" {
			return sql.NullInt64{}, invalidPriority("priority_id")
		}
		return sql.NullInt64{}, err
	}
	return sql.NullInt64{Int64: *id, Valid: true}, nil
}
