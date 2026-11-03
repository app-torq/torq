package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/app-torq/torq/internal/store"
	"github.com/app-torq/torq/internal/testutil"
)

func TestSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	d := testutil.NewDB(t)
	clk := testutil.FixedClock(t, "2026-10-30")
	users := &Users{DB: d, Clock: clk}
	u, err := users.Setup(ctx, NewUser{Username: "marek", DisplayName: "Marek", Password: "long-enough-pw", Locale: "en"})
	if err != nil {
		t.Fatal(err)
	}
	s := &Sessions{DB: d, Clock: clk}

	raw, err := s.Create(ctx, u.ID, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(b) != 32 {
		t.Fatalf("token must be 32 random bytes, got %d (%v)", len(b), err)
	}

	// Only the SHA-256 of the token is stored.
	var stored []byte
	if err := d.Reader.QueryRowContext(ctx, "SELECT token_hash FROM sessions").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(raw))
	if !bytes.Equal(stored, sum[:]) || bytes.Contains(stored, []byte(raw)) {
		t.Fatal("stored value must be sha256(token)")
	}

	got, id, err := s.Lookup(ctx, raw)
	if err != nil || got.ID != u.ID {
		t.Fatalf("lookup: %v %v", got, err)
	}

	// Lookups within an hour do not touch; after an hour the expiry slides.
	expiresBefore := expiry(t, d.Reader, id)
	clk.Advance(30 * time.Minute)
	if _, _, err := s.Lookup(ctx, raw); err != nil {
		t.Fatal(err)
	}
	if expiry(t, d.Reader, id) != expiresBefore {
		t.Fatal("session touched within the hour")
	}
	clk.Advance(31 * time.Minute)
	if _, _, err := s.Lookup(ctx, raw); err != nil {
		t.Fatal(err)
	}
	if expiry(t, d.Reader, id) == expiresBefore {
		t.Fatal("session expiry did not slide")
	}

	// Expiry after 30 days of inactivity.
	clk.Advance(SessionTTL + time.Minute)
	if _, _, err := s.Lookup(ctx, raw); err != ErrNoSession {
		t.Fatalf("expected ErrNoSession, got %v", err)
	}
	if n, err := s.PruneExpired(ctx); err != nil || n != 1 {
		t.Fatalf("prune: %d %v", n, err)
	}
}

func TestRevokeAllExcept(t *testing.T) {
	ctx := context.Background()
	d := testutil.NewDB(t)
	clk := testutil.FixedClock(t, "2026-10-30")
	u, err := (&Users{DB: d, Clock: clk}).Setup(ctx, NewUser{Username: "marek", DisplayName: "Marek", Password: "long-enough-pw", Locale: "en"})
	if err != nil {
		t.Fatal(err)
	}
	s := &Sessions{DB: d, Clock: clk}
	keep, _ := s.Create(ctx, u.ID, "", "")
	other, _ := s.Create(ctx, u.ID, "", "")
	_, keepID, err := s.Lookup(ctx, keep)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeAllExcept(ctx, u.ID, keepID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Lookup(ctx, other); err != ErrNoSession {
		t.Fatal("other session must be revoked")
	}
	if _, _, err := s.Lookup(ctx, keep); err != nil {
		t.Fatal("kept session must survive")
	}
}

func expiry(t *testing.T, q store.DBTX, id int64) string {
	t.Helper()
	var e string
	if err := q.QueryRowContext(context.Background(), "SELECT expires_at FROM sessions WHERE id = ?", id).Scan(&e); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestThrottleBackoff(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	th := NewThrottle(func() time.Time { return now }, 5)
	for i := 0; i < 4; i++ {
		th.Fail("user:a")
		if th.RetryAfter("user:a") != 0 {
			t.Fatalf("blocked after %d failures", i+1)
		}
	}
	th.Fail("user:a")
	if d := th.RetryAfter("user:a", "ip:x"); d != time.Second {
		t.Fatalf("after 5 failures wait = %v", d)
	}
	for i := 0; i < 20; i++ {
		th.Fail("user:a")
	}
	if d := th.RetryAfter("user:a"); d != maxLockout {
		t.Fatalf("lockout capped at 15m, got %v", d)
	}
	th.Succeed("user:a")
	if th.RetryAfter("user:a") != 0 {
		t.Fatal("success must reset")
	}
}

func TestClientIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	req := func(remote, xff string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = remote
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return r
	}
	cases := []struct{ remote, xff, want string }{
		{"203.0.113.5:1234", "1.2.3.4", "203.0.113.5"},                       // untrusted peer: header ignored
		{"10.0.0.2:1234", "198.51.100.7", "198.51.100.7"},                    // trusted proxy
		{"10.0.0.2:1234", "6.6.6.6, 198.51.100.7, 10.0.0.9", "198.51.100.7"}, // spoofed leftmost entry ignored
		{"10.0.0.2:1234", "", "10.0.0.2"},
	}
	for _, c := range cases {
		if got := ClientIP(req(c.remote, c.xff), trusted); got != c.want {
			t.Errorf("ClientIP(%s, %q) = %s, want %s", c.remote, c.xff, got, c.want)
		}
	}
}

func TestIsHTTPS(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	req := func(remote, proto string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = remote
		if proto != "" {
			r.Header.Set("X-Forwarded-Proto", proto)
		}
		return r
	}
	cases := []struct {
		r    *http.Request
		want bool
	}{
		{req("10.0.0.2:1234", "https"), true},
		{req("10.0.0.2:1234", "HTTPS, http"), true},
		{req("10.0.0.2:1234", "http"), false},
		{req("10.0.0.2:1234", ""), false},
		{req("203.0.113.5:1234", "https"), false}, // untrusted peer: header ignored
		{httptest.NewRequest(http.MethodGet, "https://torq.example/", nil), true},
	}
	for i, c := range cases {
		if got := IsHTTPS(c.r, trusted); got != c.want {
			t.Errorf("case %d: IsHTTPS = %v, want %v", i, got, c.want)
		}
	}
}
