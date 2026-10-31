package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const keepBackups = 3

// Migrate applies all pending forward-only migrations (research R3). When the
// database already has an applied version and migrations are pending, a copy
// is written to <dataDir>/backups first.
func Migrate(ctx context.Context, d *DB, dataDir string) error {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, d.Writer, sub)
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	pending, err := provider.HasPending(ctx)
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	if !pending {
		return nil
	}
	current, err := provider.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	if current > 0 {
		if err := backup(ctx, d, dataDir, current); err != nil {
			return fmt.Errorf("pre-migration backup: %w", err)
		}
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func backup(ctx context.Context, d *DB, dataDir string, fromVersion int64) error {
	dir := filepath.Join(dataDir, "backups")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	name := fmt.Sprintf("torq-%05d-%s.db", fromVersion, time.Now().UTC().Format("20060102T150405Z"))
	target := filepath.Join(dir, name)
	// VACUUM INTO does not accept bound parameters; the path is generated above
	// and contains no quotes, but escape defensively anyway.
	stmt := "VACUUM INTO '" + strings.ReplaceAll(target, "'", "''") + "'" //nolint:gosec // target is generated above, quotes escaped
	if _, err := d.Writer.ExecContext(ctx, stmt); err != nil {
		return err
	}
	return pruneBackups(dir)
}

func pruneBackups(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "torq-") && strings.HasSuffix(e.Name(), ".db") {
			names = append(names, e.Name())
		}
	}
	// Timestamps sort lexically; order by the timestamp suffix, not the version prefix.
	sort.Slice(names, func(i, j int) bool { return stamp(names[i]) < stamp(names[j]) })
	for len(names) > keepBackups {
		if err := os.Remove(filepath.Join(dir, names[0])); err != nil {
			return err
		}
		names = names[1:]
	}
	return nil
}

func stamp(name string) string {
	if i := strings.LastIndex(name, "-"); i >= 0 {
		return name[i+1:]
	}
	return name
}
