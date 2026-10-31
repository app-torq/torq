// Package db opens torq's SQLite database and applies migrations.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// DB holds a single-connection writer and a small reader pool on the same file
// (research R2). All writes go through Writer to avoid SQLITE_BUSY upgrades.
type DB struct {
	Writer *sql.DB
	Reader *sql.DB
	Path   string
}

// Open creates dataDir if needed and opens <dataDir>/torq.db.
func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	path := filepath.Join(dataDir, "torq.db")

	writer, err := sql.Open("sqlite", dsn(path, true))
	if err != nil {
		return nil, err
	}
	writer.SetMaxOpenConns(1)

	reader, err := sql.Open("sqlite", dsn(path, false))
	if err != nil {
		_ = writer.Close()
		return nil, err
	}
	reader.SetMaxOpenConns(4)

	// Touch the writer first so the file and WAL mode exist before readers connect.
	if err := writer.Ping(); err != nil {
		_ = writer.Close()
		_ = reader.Close()
		return nil, fmt.Errorf("open database: %w", err)
	}
	return &DB{Writer: writer, Reader: reader, Path: path}, nil
}

func dsn(path string, write bool) string {
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(ON)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "synchronous(NORMAL)")
	if write {
		// Makes BeginTx issue BEGIN IMMEDIATE so write transactions take the lock up front.
		q.Set("_txlock", "immediate")
	} else {
		q.Add("_pragma", "query_only(ON)")
	}
	return "file:" + path + "?" + q.Encode()
}

// Close closes both handles.
func (d *DB) Close() error {
	rerr := d.Reader.Close()
	if werr := d.Writer.Close(); werr != nil {
		return werr
	}
	return rerr
}

// WithTx runs fn inside a BEGIN IMMEDIATE transaction on the writer and
// commits when fn returns nil.
func (d *DB) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := d.Writer.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
