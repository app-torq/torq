package attachments

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marekbeckmann/torq/internal/apperr"
	"github.com/marekbeckmann/torq/internal/store"
	"github.com/marekbeckmann/torq/internal/testutil"
)

var (
	pngBytes  = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 64)...)
	jpegBytes = append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0}, 64)...)
	pdfBytes  = []byte("%PDF-1.7\n%âãÏÓ\n1 0 obj\n<<>>\nendobj\n")
	heicBytes = append([]byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'h', 'e', 'i', 'c'}, bytes.Repeat([]byte{0}, 64)...)
	webpBytes = append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), bytes.Repeat([]byte{0}, 64)...)
	htmlBytes = []byte("<!DOCTYPE html><script>alert(1)</script>")
	svgBytes  = []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	zipBytes  = append([]byte("PK\x03\x04"), bytes.Repeat([]byte{0}, 64)...)
)

func TestSniff(t *testing.T) {
	cases := map[string][]byte{
		"image/png": pngBytes, "image/jpeg": jpegBytes, "application/pdf": pdfBytes,
		"image/heic": heicBytes, "image/webp": webpBytes,
	}
	for want, b := range cases {
		if got := Sniff(b); got != want {
			t.Errorf("Sniff = %q, want %q", got, want)
		}
	}
	for _, b := range [][]byte{htmlBytes, svgBytes, zipBytes} {
		if allowed[Sniff(b)] {
			t.Errorf("disallowed content sniffed as %q", Sniff(b))
		}
	}
}

func newStore(t *testing.T, maxBytes int64) (*Store, int64) {
	t.Helper()
	d := testutil.NewDB(t)
	clk := testutil.FixedClock(t, "2026-10-30")
	ctx := context.Background()
	w := store.New(d.Writer)
	v, err := w.CreateVehicle(ctx, store.CreateVehicleParams{Make: "BMW", Model: "330d", CurrentOdometerKm: 1, CreatedAt: "x", UpdatedAt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	l, err := w.CreateServiceLog(ctx, store.CreateServiceLogParams{VehicleID: v.ID, ServiceDate: "2026-10-01", OdometerKm: 1, CreatedAt: "x", UpdatedAt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	return &Store{DB: d, Clock: clk, Dir: filepath.Join(t.TempDir(), "attachments"), MaxBytes: maxBytes, MaxMB: 1, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}, l.ID
}

func TestSaveRejectsDisguisedAndOversize(t *testing.T) {
	s, logID := newStore(t, 1<<20)
	ctx := context.Background()
	for _, b := range [][]byte{htmlBytes, svgBytes, zipBytes} {
		_, err := s.Save(ctx, bytes.NewReader(b), "photo.png", Owner{ServiceLogID: logID}, PurposeFile, "", 0)
		if e, ok := apperr.As(err); !ok || e.Code != "attachment.type_not_allowed" || e.Status != 415 {
			t.Fatalf("got %v", err)
		}
	}
	big := append(append([]byte{}, pdfBytes...), bytes.Repeat([]byte("x"), 1<<20)...)
	_, err := s.Save(ctx, bytes.NewReader(big), "manual.pdf", Owner{ServiceLogID: logID}, PurposeFile, "", 0)
	if e, ok := apperr.As(err); !ok || e.Code != "attachment.too_large" || e.Params["max_mb"] != 1 {
		t.Fatalf("got %v", err)
	}
	// Spec-entry and avatar uploads must be images.
	if _, err := s.Save(ctx, bytes.NewReader(pdfBytes), "x.pdf", Owner{VehicleID: 1}, PurposeAvatar, "", 0); err == nil {
		t.Fatal("pdf avatar accepted")
	}
	// Nothing left behind in tmp.
	entries, _ := os.ReadDir(filepath.Join(s.Dir, "tmp"))
	if len(entries) != 0 {
		t.Fatalf("temp files left: %d", len(entries))
	}
}

func TestSaveDeleteCascadeAndSweep(t *testing.T) {
	s, logID := newStore(t, 1<<20)
	ctx := context.Background()
	a, err := s.Save(ctx, bytes.NewReader(pdfBytes), "../../etc/receipt.pdf", Owner{ServiceLogID: logID}, PurposeFile, "receipt", 0)
	if err != nil {
		t.Fatal(err)
	}
	if a.OriginalName != "receipt.pdf" || a.MimeType != "application/pdf" || a.SizeBytes != int64(len(pdfBytes)) {
		t.Fatalf("saved %+v", a)
	}
	if _, err := os.Stat(s.path(a.StorageKey)); err != nil {
		t.Fatal(err)
	}

	// Owner deletion: collect keys in the tx, remove files after commit.
	var keys []string
	err = s.DB.WithTx(ctx, func(tx *sql.Tx) error {
		var err error
		if keys, err = s.KeysForServiceLog(ctx, tx, logID); err != nil {
			return err
		}
		return store.New(tx).DeleteServiceLog(ctx, logID)
	})
	if err != nil {
		t.Fatal(err)
	}
	s.RemoveFiles(keys)
	if _, err := os.Stat(s.path(a.StorageKey)); !os.IsNotExist(err) {
		t.Fatal("file not removed with its owner")
	}

	// Orphan sweep removes unreferenced old files only.
	orphan := filepath.Join(s.Dir, "2026", "10", "deadbeef")
	if err := os.WriteFile(orphan, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.SweepOrphans(ctx, time.Hour); n != 0 {
		t.Fatal("fresh orphan swept")
	}
	old := time.Now().Add(-2 * time.Hour)
	_ = os.Chtimes(orphan, old, old)
	if n, err := s.SweepOrphans(ctx, time.Hour); err != nil || n != 1 {
		t.Fatalf("sweep %d %v", n, err)
	}
}
