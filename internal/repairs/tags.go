package repairs

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/db"
	"github.com/app-torq/torq/internal/store"
)

// MaxTags is how many tags one project can have.
const MaxTags = 50

// Tag is a project tag with the number of projects using it.
type Tag struct {
	store.RepairTag
	ProjectCount int64
}

// normalizeTag trims a tag name and collapses inner whitespace.
func normalizeTag(name string) string { return strings.Join(strings.Fields(name), " ") }

// tagError reports why name cannot be a tag ("" when it can). Commas and
// double quotes are kept out because the search syntax uses them.
func tagError(name string) string {
	n := utf8.RuneCountInString(name)
	switch {
	case n < 1 || n > 40:
		return "validation.length"
	case strings.ContainsAny(name, `,"`):
		return "repair.tag_invalid"
	}
	return ""
}

// normalizeTags cleans a project's tag list: names are normalized, and names
// that differ only in case count once.
func normalizeTags(names []string, v *apperr.Validation) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = normalizeTag(name)
		if name == "" || containsFold(out, name) {
			continue
		}
		if code := tagError(name); code != "" {
			v.Add("tags", code)
			continue
		}
		out = append(out, name)
	}
	v.Check(len(out) > MaxTags, "tags", "repair.too_many_tags")
	return out
}

func containsFold(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

// findTag returns the tag whose name matches name regardless of case.
// strings.EqualFold also folds non-ASCII letters, which SQLite's NOCASE does not.
func findTag(tags []store.RepairTag, name string) (store.RepairTag, bool) {
	for _, t := range tags {
		if strings.EqualFold(t.Name, name) {
			return t, true
		}
	}
	return store.RepairTag{}, false
}

// writeTags replaces a project's tags, creating tags that don't exist yet.
// An existing tag keeps its spelling.
func (s *Service) writeTags(ctx context.Context, q *store.Queries, id int64, names []string) error {
	if err := q.DeleteRepairLogTags(ctx, id); err != nil {
		return err
	}
	if len(names) == 0 {
		return nil
	}
	all, err := q.AllRepairTags(ctx)
	if err != nil {
		return err
	}
	now := clock.Timestamp(s.Clock.Now())
	for _, name := range names {
		tag, ok := findTag(all, name)
		if !ok {
			if tag, err = q.CreateRepairTag(ctx, store.CreateRepairTagParams{Name: name, CreatedAt: now, UpdatedAt: now}); err != nil {
				return err
			}
			all = append(all, tag)
		}
		if err := q.InsertRepairLogTag(ctx, store.InsertRepairLogTagParams{RepairLogID: id, TagID: tag.ID}); err != nil {
			return err
		}
	}
	return nil
}

// tagsFor returns the tag names of each project, sorted by name.
func tagsFor(ctx context.Context, q *store.Queries, ids []int64) (map[int64][]string, error) {
	rows, err := q.ListRepairLogTags(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := map[int64][]string{}
	for _, r := range rows {
		out[r.RepairLogID] = append(out[r.RepairLogID], r.Name)
	}
	return out, nil
}

// tagFilter is a set of tag ids for the list query.
type tagFilter struct {
	joined string // comma-separated ids
	count  int64
}

// resolveTags looks up tag names regardless of case. known is false when a
// name matches no tag, so no project can match.
func resolveTags(ctx context.Context, q *store.Queries, names []string) (tagFilter, bool, error) {
	if len(names) == 0 {
		return tagFilter{}, true, nil
	}
	all, err := q.AllRepairTags(ctx)
	if err != nil {
		return tagFilter{}, false, err
	}
	seen := map[int64]bool{}
	var ids []string
	for _, name := range names {
		name = normalizeTag(name)
		if name == "" {
			continue
		}
		tag, ok := findTag(all, name)
		if !ok {
			return tagFilter{}, false, nil
		}
		if !seen[tag.ID] {
			seen[tag.ID] = true
			ids = append(ids, strconv.FormatInt(tag.ID, 10))
		}
	}
	return tagFilter{joined: strings.Join(ids, ","), count: int64(len(ids))}, true, nil
}

// ListTags returns tags by name. With visible vehicles (nil = all), only
// tags on their projects are listed and counted, so tag names used only on
// other vehicles stay hidden.
func (s *Service) ListTags(ctx context.Context, visible []int64) ([]Tag, error) {
	rows, err := store.New(s.DB.Reader).ListRepairTags(ctx, db.Scope(visible))
	if err != nil {
		return nil, err
	}
	out := make([]Tag, 0, len(rows))
	for _, r := range rows {
		out = append(out, Tag{RepairTag: store.RepairTag{ID: r.ID, Name: r.Name, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}, ProjectCount: r.ProjectCount})
	}
	return out, nil
}

// RenameTag renames a tag on every project. Another tag's name, in any case,
// is a duplicate.
func (s *Service) RenameTag(ctx context.Context, id int64, name string) (store.RepairTag, error) {
	name = normalizeTag(name)
	var v apperr.Validation
	if code := tagError(name); code != "" {
		v.Add("name", code)
		return store.RepairTag{}, v.Err()
	}
	var out store.RepairTag
	err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if _, err := q.GetRepairTag(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		} else if err != nil {
			return err
		}
		all, err := q.AllRepairTags(ctx)
		if err != nil {
			return err
		}
		if other, ok := findTag(all, name); ok && other.ID != id {
			v.Add("name", "validation.duplicate")
			return v.Err()
		}
		out, err = q.RenameRepairTag(ctx, store.RenameRepairTagParams{Name: name, UpdatedAt: clock.Timestamp(s.Clock.Now()), ID: id})
		return err
	})
	return out, err
}

// DeleteTag removes a tag from every project.
func (s *Service) DeleteTag(ctx context.Context, id int64) error {
	return s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if _, err := q.GetRepairTag(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return apperr.NotFound()
		} else if err != nil {
			return err
		}
		return q.DeleteRepairTag(ctx, id)
	})
}
