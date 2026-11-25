package mfa

import (
	"bytes"
	"regexp"
	"testing"
)

func TestRecoveryCodes(t *testing.T) {
	codes := NewRecoveryCodes()
	if len(codes) != RecoveryCodeCount {
		t.Fatalf("%d codes", len(codes))
	}
	format := regexp.MustCompile(`^[2-9a-hjkmnp-z]{5}-[2-9a-hjkmnp-z]{5}$`)
	seen := map[string]bool{}
	for _, c := range codes {
		if !format.MatchString(c) || seen[c] {
			t.Errorf("code %q", c)
		}
		seen[c] = true
	}
	if !bytes.Equal(HashRecoveryCode(codes[0]), HashRecoveryCode(" "+codes[0][:5]+codes[0][6:]+" ")) {
		t.Error("typed without dash")
	}
	upper := []byte(codes[0])
	for i, c := range upper {
		if c >= 'a' && c <= 'z' {
			upper[i] = c - 32
		}
	}
	if !bytes.Equal(HashRecoveryCode(codes[0]), HashRecoveryCode(string(upper))) {
		t.Error("typed in capitals")
	}
}
