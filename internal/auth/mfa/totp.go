// Package mfa holds the second factors of the authentication boundary
// (Constitution IV): time-based codes, recovery codes, passkeys and the
// short-lived sign-in challenges. Only internal/auth and internal/httpapi
// import it.
package mfa

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 uses HMAC-SHA1, which collisions don't affect
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Time-based codes (RFC 6238): six digits for 30-second steps.
const (
	totpDigits = 6
	totpPeriod = 30
	secretSize = 20
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret returns a random secret.
func NewTOTPSecret() []byte {
	b := make([]byte, secretSize)
	_, _ = rand.Read(b) // crypto/rand never fails
	return b
}

// SecretText is the secret as people type it into an authenticator app.
func SecretText(secret []byte) string { return b32.EncodeToString(secret) }

// TOTPURI is the otpauth:// link a QR code carries.
func TOTPURI(issuer, account string, secret []byte) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	q := url.Values{}
	q.Set("secret", SecretText(secret))
	q.Set("issuer", issuer)
	q.Set("digits", fmt.Sprint(totpDigits))
	q.Set("period", fmt.Sprint(totpPeriod))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// Step is the 30-second interval t falls in.
func Step(t time.Time) int64 { return t.Unix() / totpPeriod }

// CodeAt returns the code for a step.
func CodeAt(secret []byte, step int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step)) //nolint:gosec // steps are positive
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", n%1_000_000)
}

// MatchStep returns the step a code belongs to, accepting one step of clock
// drift either way, and only steps after lastStep (each code works once).
func MatchStep(secret []byte, code string, now time.Time, lastStep int64) (int64, bool) {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != totpDigits {
		return 0, false
	}
	cur := Step(now)
	for _, s := range []int64{cur - 1, cur, cur + 1} {
		if s > lastStep && subtle.ConstantTimeCompare([]byte(CodeAt(secret, s)), []byte(code)) == 1 {
			return s, true
		}
	}
	return 0, false
}
