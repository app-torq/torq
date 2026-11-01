package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/pressly/goose/v3"
)

func TestOpenAndMigrate(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if err := Migrate(ctx, d, dir); err != nil {
		t.Fatal(err)
	}
	var mode string
	if err := d.Reader.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q", mode)
	}
	var fk int
	if err := d.Writer.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys = %d", fk)
	}

	// Second run is a no-op and creates no backup.
	if err := Migrate(ctx, d, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "backups")); !os.IsNotExist(err) {
		t.Fatalf("unexpected backups dir: %v", err)
	}
}

func TestPendingMigrationCreatesBackup(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	d, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := Migrate(ctx, d, dir); err != nil {
		t.Fatal(err)
	}

	// Simulate a newer release with one extra migration.
	extra := fstest.MapFS{"99999_extra.sql": {Data: []byte("-- +goose Up\nCREATE TABLE extra (id INTEGER PRIMARY KEY);\n")}}
	p, err := goose.NewProvider(goose.DialectSQLite3, d.Writer, extra, goose.WithAllowOutofOrder(true))
	if err != nil {
		t.Fatal(err)
	}
	current, err := p.GetDBVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := backup(ctx, d, dir, current); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(entries))
	}
}

func TestPruneBackupsKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{
		"torq-00001-20260101T000000Z.db",
		"torq-00002-20260102T000000Z.db",
		"torq-00003-20260103T000000Z.db",
		"torq-00004-20260104T000000Z.db",
	} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := pruneBackups(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "torq-00001-20260101T000000Z.db")); !os.IsNotExist(err) {
		t.Fatal("oldest backup should be pruned")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != keepBackups {
		t.Fatalf("expected %d backups, got %d", keepBackups, len(entries))
	}
}
