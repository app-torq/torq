package mfa

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestBox(t *testing.T) {
	dir := t.TempDir()
	b, err := OpenBox(dir)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, KeyFile))
	if err != nil || info.Mode().Perm() != 0o600 || info.Size() != 32 {
		t.Fatalf("key file %v %v", info, err)
	}
	sealed := b.Seal(7, []byte("secret"))
	if bytes.Contains(sealed, []byte("secret")) {
		t.Fatal("plain text in the sealed value")
	}
	// The key survives a restart.
	again, err := OpenBox(dir)
	if err != nil {
		t.Fatal(err)
	}
	if plain, err := again.Open(7, sealed); err != nil || string(plain) != "secret" {
		t.Fatalf("open %q %v", plain, err)
	}
	if _, err := again.Open(8, sealed); err == nil {
		t.Fatal("opened another user's secret")
	}
	if err := os.WriteFile(filepath.Join(dir, KeyFile), []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenBox(dir); err == nil {
		t.Fatal("accepted a short key")
	}
}
