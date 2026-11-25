package vehicles

import (
	"encoding/base64"
	"regexp"
	"strings"
	"testing"

	"github.com/app-torq/torq/internal/apperr"
)

func TestShareToken(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		tok, err := NewShareToken()
		if err != nil {
			t.Fatal(err)
		}
		if !regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`).MatchString(tok) {
			t.Fatalf("token %q", tok)
		}
		if b, err := base64.RawURLEncoding.DecodeString(tok); err != nil || len(b) != 32 {
			t.Fatalf("token %q decodes to %d bytes (%v)", tok, len(b), err)
		}
		if seen[tok] {
			t.Fatal("duplicate token")
		}
		seen[tok] = true
	}
	if len(ShareHash("x")) != 32 {
		t.Fatal("hash length")
	}
}

func isNotFound(err error) bool {
	e, ok := apperr.As(err)
	return ok && e.Status == 404
}

func TestShareLifecycle(t *testing.T) {
	s, ctx := newService(t)
	v, err := s.Create(ctx, Profile{Make: "BMW", Model: "330d"}, 1000, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Share(ctx, v.ID); !isNotFound(err) {
		t.Fatalf("no link yet: %v", err)
	}

	sh, created, err := s.EnableShare(ctx, v.ID, ShareOptions{IncludeVIN: true}, 0)
	if err != nil || !created || sh.IncludeVin != 1 || sh.IncludeCosts != 0 {
		t.Fatalf("enable %+v %v %v", sh, created, err)
	}
	// Changing options keeps the token.
	again, created, err := s.EnableShare(ctx, v.ID, ShareOptions{IncludeCosts: true}, 0)
	if err != nil || created || again.Token != sh.Token || again.IncludeCosts != 1 || again.IncludeVin != 0 {
		t.Fatalf("options %+v %v %v", again, created, err)
	}
	if got, err := s.ResolveShare(ctx, sh.Token); err != nil || got.VehicleID != v.ID {
		t.Fatalf("resolve %+v %v", got, err)
	}
	// Change the last character to one it isn't: it carries only 4 bits, so a
	// fixed replacement would match the real token every 16th run.
	last := "A"
	if strings.HasSuffix(sh.Token, last) {
		last = "Q"
	}
	for _, bad := range []string{"", "x", sh.Token + "x", sh.Token[:42] + last, "../../etc/passwd"} {
		if _, err := s.ResolveShare(ctx, bad); !isNotFound(err) {
			t.Errorf("resolve %q: %v", bad, err)
		}
	}

	// Renewing replaces the token.
	renewed, err := s.RenewShare(ctx, v.ID, 0)
	if err != nil || renewed.Token == sh.Token {
		t.Fatalf("renew %+v %v", renewed, err)
	}
	if _, err := s.ResolveShare(ctx, sh.Token); !isNotFound(err) {
		t.Fatal("old token still works")
	}

	// Archiving pauses the link; restoring brings the same token back.
	if _, err := s.SetArchived(ctx, v.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ResolveShare(ctx, renewed.Token); !isNotFound(err) {
		t.Fatal("archived vehicle still shared")
	}
	if _, err := s.SetArchived(ctx, v.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ResolveShare(ctx, renewed.Token); err != nil {
		t.Fatalf("restored vehicle: %v", err)
	}

	// The instance switch stops every link and blocks new ones.
	if _, err := s.SetPublicLinks(ctx, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ResolveShare(ctx, renewed.Token); !isNotFound(err) {
		t.Fatal("link works with sharing off")
	}
	if _, _, err := s.EnableShare(ctx, v.ID, ShareOptions{}, 0); err == nil {
		t.Fatal("enabled with sharing off")
	} else if e, ok := apperr.As(err); !ok || e.Code != "share.disabled" {
		t.Fatalf("sharing off: %v", err)
	}
	if _, err := s.SetPublicLinks(ctx, true); err != nil {
		t.Fatal(err)
	}

	// Turning off, then on again, issues a new token.
	if err := s.DisableShare(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ResolveShare(ctx, renewed.Token); !isNotFound(err) {
		t.Fatal("disabled link works")
	}
	if _, err := s.RenewShare(ctx, v.ID, 0); !isNotFound(err) {
		t.Fatalf("renew without link: %v", err)
	}
	fresh, _, err := s.EnableShare(ctx, v.ID, ShareOptions{}, 0)
	if err != nil || fresh.Token == renewed.Token || fresh.Token == sh.Token {
		t.Fatalf("re-enable %+v %v", fresh, err)
	}
	if err := s.TouchShare(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Share(ctx, v.ID); !got.LastOpenedAt.Valid {
		t.Fatal("view not recorded")
	}
}
