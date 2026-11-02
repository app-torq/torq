// Package servicelog owns the Service Logbook: routine interval maintenance
// sessions and their categories. It must not import the repairs package
// (Constitution II).
package servicelog

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/store"
)

// CategoryInput creates or updates a category. Built-in categories accept only
// the default intervals.
type CategoryInput struct {
	CustomName            *string
	IsFluid               *bool
	DefaultIntervalKm     *int64
	DefaultIntervalMonths *int64
}

func nullInt(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func validateIntervals(in CategoryInput, v *apperr.Validation) {
	v.Check(in.DefaultIntervalKm != nil && *in.DefaultIntervalKm <= 0, "default_interval_km", "validation.positive")
	v.Check(in.DefaultIntervalMonths != nil && (*in.DefaultIntervalMonths < 1 || *in.DefaultIntervalMonths > 240), "default_interval_months", "validation.range")
}

func mapUnique(err error) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		var v apperr.Validation
		v.Add("custom_name", "validation.duplicate")
		return v.Err()
	}
	return err
}

// ListCategories returns categories, optionally including archived ones.
func (s *Service) ListCategories(ctx context.Context, includeArchived bool) ([]store.ServiceCategory, error) {
	var inc int64
	if includeArchived {
		inc = 1
	}
	return store.New(s.DB.Reader).ListCategories(ctx, inc)
}

func getCategory(ctx context.Context, q *store.Queries, id int64) (store.ServiceCategory, error) {
	c, err := q.GetCategory(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return store.ServiceCategory{}, apperr.NotFound()
	}
	return c, err
}

// CreateCategory adds a custom category.
func (s *Service) CreateCategory(ctx context.Context, in CategoryInput) (store.ServiceCategory, error) {
	var v apperr.Validation
	name := ""
	if in.CustomName != nil {
		name = strings.TrimSpace(*in.CustomName)
	}
	n := utf8.RuneCountInString(name)
	v.Check(n < 1 || n > 64, "custom_name", "validation.length")
	validateIntervals(in, &v)
	if err := v.Err(); err != nil {
		return store.ServiceCategory{}, err
	}
	fluid := int64(0)
	if in.IsFluid != nil && *in.IsFluid {
		fluid = 1
	}
	c, err := store.New(s.DB.Writer).CreateCustomCategory(ctx, store.CreateCustomCategoryParams{
		CustomName: sql.NullString{String: name, Valid: true}, IsFluid: fluid,
		DefaultIntervalKm: nullInt(in.DefaultIntervalKm), DefaultIntervalMonths: nullInt(in.DefaultIntervalMonths),
	})
	return c, mapUnique(err)
}

// UpdateCategory edits a category. Built-ins accept only default intervals;
// nil intervals clear them.
func (s *Service) UpdateCategory(ctx context.Context, id int64, in CategoryInput) (store.ServiceCategory, error) {
	q := store.New(s.DB.Writer)
	cur, err := getCategory(ctx, q, id)
	if err != nil {
		return store.ServiceCategory{}, err
	}
	var v apperr.Validation
	validateIntervals(in, &v)
	name, fluid := cur.CustomName, cur.IsFluid
	if cur.BuiltinKey.Valid {
		v.Check(in.CustomName != nil || in.IsFluid != nil, "custom_name", "category.builtin_readonly")
	} else {
		if in.CustomName != nil {
			trimmed := strings.TrimSpace(*in.CustomName)
			n := utf8.RuneCountInString(trimmed)
			v.Check(n < 1 || n > 64, "custom_name", "validation.length")
			name = sql.NullString{String: trimmed, Valid: true}
		}
		if in.IsFluid != nil {
			fluid = 0
			if *in.IsFluid {
				fluid = 1
			}
		}
	}
	if err := v.Err(); err != nil {
		return store.ServiceCategory{}, err
	}
	c, err := q.UpdateCategory(ctx, store.UpdateCategoryParams{
		CustomName: name, IsFluid: fluid,
		DefaultIntervalKm: nullInt(in.DefaultIntervalKm), DefaultIntervalMonths: nullInt(in.DefaultIntervalMonths), ID: id,
	})
	return c, mapUnique(err)
}

// SetCategoryArchived archives or restores a custom category.
func (s *Service) SetCategoryArchived(ctx context.Context, id int64, archived bool) (store.ServiceCategory, error) {
	q := store.New(s.DB.Writer)
	cur, err := getCategory(ctx, q, id)
	if err != nil {
		return store.ServiceCategory{}, err
	}
	if cur.BuiltinKey.Valid {
		return store.ServiceCategory{}, apperr.Conflict("category.builtin_readonly", nil)
	}
	at := sql.NullString{}
	if archived {
		at = sql.NullString{String: clock.Timestamp(s.Clock.Now()), Valid: true}
	}
	return q.SetCategoryArchived(ctx, store.SetCategoryArchivedParams{ArchivedAt: at, ID: id})
}

// DeleteCategory removes an unused custom category.
func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
	return s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		cur, err := getCategory(ctx, q, id)
		if err != nil {
			return err
		}
		if cur.BuiltinKey.Valid {
			return apperr.Conflict("category.builtin_readonly", nil)
		}
		inUse, err := q.CategoryInUse(ctx, id)
		if err != nil {
			return err
		}
		if inUse != 0 {
			return apperr.Conflict("category.in_use", nil)
		}
		return q.DeleteCategory(ctx, id)
	})
}
