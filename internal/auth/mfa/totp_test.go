package mfa

import (
	"strings"
	"testing"
	"time"
)

// RFC 6238 Appendix B (SHA-1 secret), last six digits of the eight-digit values.
func TestCodeAtRFC6238(t *testing.T) {
	secret := []byte("12345678901234567890")
	for _, c := range []struct {
		unix int64
		code string
	}{
		{59, "287082"}, {1111111109, "081804"}, {1111111111, "050471"},
		{1234567890, "005924"}, {2000000000, "279037"}, {20000000000, "353130"},
	} {
		if got := CodeAt(secret, Step(time.Unix(c.unix, 0))); got != c.code {
			t.Errorf("%d: %s, want %s", c.unix, got, c.code)
		}
	}
}

func TestMatchStep(t *testing.T) {
	secret := NewTOTPSecret()
	now := time.Unix(1_800_000_000, 0)
	cur := Step(now)
	for _, c := range []struct {
		name string
		step int64
		last int64
		ok   bool
	}{
		{"current", cur, 0, true},
		{"previous step", cur - 1, 0, true},
		{"next step", cur + 1, 0, true},
		{"two steps old", cur - 2, 0, false},
		{"already used", cur, cur, false},
		{"older than the last used", cur - 1, cur, false},
	} {
		got, ok := MatchStep(secret, CodeAt(secret, c.step), now, c.last)
		if ok != c.ok || (ok && got != c.step) {
			t.Errorf("%s: %d %v", c.name, got, ok)
		}
	}
	if _, ok := MatchStep(secret, "12345", now, 0); ok {
		t.Error("accepted five digits")
	}
	code := CodeAt(secret, cur)
	if _, ok := MatchStep(secret, " "+code[:3]+" "+code[3:]+" ", now, 0); !ok {
		t.Error("spaces in a typed code")
	}
}

func TestTOTPURI(t *testing.T) {
	uri := TOTPURI("Garage Nord", "anna", []byte("12345678901234567890"))
	for _, want := range []string{"otpauth://totp/Garage%20Nord:anna?", "secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "issuer=Garage+Nord", "digits=6", "period=30"} {
		if !strings.Contains(uri, want) {
			t.Errorf("%s lacks %s", uri, want)
		}
	}
}
