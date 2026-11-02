// Package testutil provides shared helpers for tests.
package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/db"
)

// NewDB opens a fresh, fully migrated database in a temp directory.
func NewDB(t testing.TB) *db.DB {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := db.Migrate(context.Background(), d, dir); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return d
}

// FixedClock returns a clock pinned to noon local time on date (YYYY-MM-DD).
func FixedClock(t testing.TB, date string) *clock.Fixed {
	t.Helper()
	d, err := time.ParseInLocation(clock.DateLayout, date, time.Local)
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}
	return &clock.Fixed{At: d.Add(12 * time.Hour)}
}
