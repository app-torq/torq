// Package attachments stores uploaded images and PDFs on the data volume
// (research R18). Metadata lives in the attachments table; files live under
// <data>/attachments/<yyyy>/<mm>/<random hex>.
package attachments

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/marekbeckmann/torq/internal/apperr"
	"github.com/marekbeckmann/torq/internal/clock"
	"github.com/marekbeckmann/torq/internal/db"
	"github.com/marekbeckmann/torq/internal/store"
)

// Purposes.
const (
	PurposeFile   = "file"
	PurposeAvatar = "avatar"
)

// Store saves, serves and deletes attachment files.
type Store struct {
	DB       *db.DB
	Clock    clock.Clock
	Dir      string // <data>/attachments
	MaxBytes int64
	MaxMB    int
	Log      *slog.Logger
}

// Owner identifies the single record an attachment belongs to.
type Owner struct {
	VehicleID    int64 // avatar only
	ServiceLogID int64
	RepairLogID  int64
	SpecEntryID  int64
}

func nullID(id int64) sql.NullInt64 { return sql.NullInt64{Int64: id, Valid: id > 0} }

// heifBrands are ISO-BMFF major brands used by HEIC/HEIF photos.
var heifBrands = map[string]string{
	"heic": "image/heic", "heix": "image/heic", "hevc": "image/heic", "heim": "image/heic", "heis": "image/heic",
	"mif1": "image/heif", "msf1": "image/heif", "heif": "image/heif",
}

var allowed = map[string]bool{
	"image/jpeg": true, "image/png": true, "image/webp": true,
	"image/heic": true, "image/heif": true, "application/pdf": true,
}

// Sniff detects the content type from the first bytes. It never trusts the
// client-supplied type, so disguised HTML/SVG uploads are rejected.
func Sniff(head []byte) string {
	if len(head) >= 12 && string(head[4:8]) == "ftyp" {
		if mt, ok := heifBrands[string(head[8:12])]; ok {
			return mt
		}
	}
	mt := http.DetectContentType(head)
	if i := strings.IndexByte(mt, ';'); i >= 0 {
		mt = mt[:i]
	}
	return mt
}

// IsImage reports whether mt is an allowed image type.
func IsImage(mt string) bool { return strings.HasPrefix(mt, "image/") && allowed[mt] }

// SanitizeName strips path components and control characters (max 255 runes).
func SanitizeName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	for utf8.RuneCountInString(name) > 255 {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	if name == "" {
		name = "file"
	}
	return name
}

func (s *Store) path(key string) string { return filepath.Join(s.Dir, filepath.FromSlash(key)) }

// Save streams r to disk, validates type and size, and inserts the metadata
// row. For avatars, the previous avatar of the vehicle is replaced.
func (s *Store) Save(ctx context.Context, r io.Reader, originalName string, owner Owner, purpose, caption string, uploadedBy int64) (store.Attachment, error) {
	if err := os.MkdirAll(filepath.Join(s.Dir, "tmp"), 0o750); err != nil {
		return store.Attachment{}, err
	}
	tmp, err := os.CreateTemp(filepath.Join(s.Dir, "tmp"), "upload-*")
	if err != nil {
		return store.Attachment{}, err
	}
	tmpName := tmp.Name()
	keep := false
	defer func() {
		_ = tmp.Close()
		if !keep {
			_ = os.Remove(tmpName)
		}
	}()

	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return store.Attachment{}, s.readErr(err)
	}
	head = head[:n]
	if n == 0 {
		return store.Attachment{}, apperr.Unprocessable("attachment.empty", nil)
	}
	mt := Sniff(head)
	if !allowed[mt] || ((purpose == PurposeAvatar || owner.SpecEntryID > 0) && !IsImage(mt)) {
		return store.Attachment{}, apperr.Unsupported("attachment.type_not_allowed")
	}
	written, err := io.Copy(tmp, io.MultiReader(bytes.NewReader(head), io.LimitReader(r, s.MaxBytes+1-int64(n))))
	if err != nil {
		return store.Attachment{}, s.readErr(err)
	}
	if written > s.MaxBytes {
		return store.Attachment{}, apperr.TooLarge(s.MaxMB)
	}
	if err := tmp.Close(); err != nil {
		return store.Attachment{}, err
	}

	var rnd [16]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return store.Attachment{}, err
	}
	now := s.Clock.Now()
	key := fmt.Sprintf("%04d/%02d/%s", now.Year(), int(now.Month()), hex.EncodeToString(rnd[:]))
	final := s.path(key)
	if err := os.MkdirAll(filepath.Dir(final), 0o750); err != nil {
		return store.Attachment{}, err
	}
	if err := os.Rename(tmpName, final); err != nil {
		return store.Attachment{}, err
	}
	keep = true

	var out store.Attachment
	var replaced []string
	err = s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		q := store.New(tx)
		if purpose == PurposeAvatar {
			prev, err := q.GetVehicleAvatar(ctx, nullID(owner.VehicleID))
			if err == nil {
				replaced = append(replaced, prev.StorageKey)
				if err := q.DeleteAttachment(ctx, prev.ID); err != nil {
					return err
				}
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		var err error
		out, err = q.CreateAttachment(ctx, store.CreateAttachmentParams{
			StorageKey: key, OriginalName: SanitizeName(originalName), MimeType: mt, SizeBytes: written,
			Purpose: purpose, VehicleID: nullID(owner.VehicleID), ServiceLogID: nullID(owner.ServiceLogID),
			RepairLogID: nullID(owner.RepairLogID), SpecEntryID: nullID(owner.SpecEntryID),
			Caption: strings.TrimSpace(caption), UploadedBy: nullID(uploadedBy), CreatedAt: clock.Timestamp(now),
		})
		return err
	})
	if err != nil {
		_ = os.Remove(final)
		if strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
			return store.Attachment{}, apperr.NotFound()
		}
		return store.Attachment{}, err
	}
	s.RemoveFiles(replaced)
	return out, nil
}

func (s *Store) readErr(err error) error {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return apperr.TooLarge(s.MaxMB)
	}
	return err
}

// Get returns attachment metadata.
func (s *Store) Get(ctx context.Context, id int64) (store.Attachment, error) {
	a, err := store.New(s.DB.Reader).GetAttachment(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return store.Attachment{}, apperr.NotFound()
	}
	return a, err
}

// Open returns the metadata and an open file for streaming.
func (s *Store) Open(ctx context.Context, id int64) (store.Attachment, *os.File, error) {
	a, err := s.Get(ctx, id)
	if err != nil {
		return a, nil, err
	}
	f, err := os.Open(s.path(a.StorageKey))
	if errors.Is(err, fs.ErrNotExist) {
		return a, nil, apperr.NotFound()
	}
	return a, f, err
}

// Delete removes the row and then the file.
func (s *Store) Delete(ctx context.Context, id int64) error {
	a, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := store.New(s.DB.Writer).DeleteAttachment(ctx, id); err != nil {
		return err
	}
	s.RemoveFiles([]string{a.StorageKey})
	return nil
}

// UpdateCaption changes the caption.
func (s *Store) UpdateCaption(ctx context.Context, id int64, caption string) (store.Attachment, error) {
	caption = strings.TrimSpace(caption)
	if utf8.RuneCountInString(caption) > 120 {
		var v apperr.Validation
		v.Add("caption", "validation.length")
		return store.Attachment{}, v.Err()
	}
	a, err := store.New(s.DB.Writer).UpdateAttachmentCaption(ctx, store.UpdateAttachmentCaptionParams{Caption: caption, ID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return store.Attachment{}, apperr.NotFound()
	}
	return a, err
}

// RemoveFiles deletes files by storage key; failures are logged and left for the sweep.
func (s *Store) RemoveFiles(keys []string) {
	for _, k := range keys {
		if err := os.Remove(s.path(k)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			s.Log.Warn("remove attachment file", "key", k, "err", err)
		}
	}
}

// Collectors return storage keys to delete after the owner row is removed.

func (s *Store) KeysForVehicle(ctx context.Context, tx *sql.Tx, id int64) ([]string, error) {
	return store.New(tx).StorageKeysForVehicle(ctx, nullID(id))
}

func (s *Store) KeysForServiceLog(ctx context.Context, tx *sql.Tx, id int64) ([]string, error) {
	return store.New(tx).StorageKeysForServiceLog(ctx, nullID(id))
}

func (s *Store) KeysForRepair(ctx context.Context, tx *sql.Tx, id int64) ([]string, error) {
	return store.New(tx).StorageKeysForRepair(ctx, nullID(id))
}

func (s *Store) KeysForSpecEntry(ctx context.Context, tx *sql.Tx, id int64) ([]string, error) {
	return store.New(tx).StorageKeysForSpecEntry(ctx, nullID(id))
}

// SweepOrphans removes files older than olderThan that no row references,
// including abandoned temp uploads.
func (s *Store) SweepOrphans(ctx context.Context, olderThan time.Duration) (int, error) {
	keys, err := store.New(s.DB.Reader).ListStorageKeys(ctx)
	if err != nil {
		return 0, err
	}
	known := make(map[string]bool, len(keys))
	for _, k := range keys {
		known[k] = true
	}
	// File mtimes are wall-clock time, so compare against the real clock.
	cutoff := time.Now().Add(-olderThan)
	removed := 0
	root, err := os.OpenRoot(s.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer func() { _ = root.Close() }()
	err = filepath.WalkDir(s.Dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(s.Dir, p)
		if err != nil {
			return err
		}
		if known[filepath.ToSlash(rel)] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().Before(cutoff) {
			// Remove through the root so a symlink cannot redirect the delete outside the data dir.
			if err := root.Remove(rel); err == nil {
				removed++
			}
		}
		return nil
	})
	return removed, err
}
