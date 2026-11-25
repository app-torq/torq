package mfa

import (
	"crypto/rand"
	"crypto/sha256"
	"strings"
)

// RecoveryCodeCount is how many codes a set has.
const RecoveryCodeCount = 10

// alphabet leaves out characters that are easy to mix up (0/o, 1/l/i).
const alphabet = "23456789abcdefghjkmnpqrstuvwxyz"

// NewRecoveryCodes returns a new set, formatted as xxxxx-xxxxx (~49 bits each).
func NewRecoveryCodes() []string {
	out := make([]string, RecoveryCodeCount)
	buf := make([]byte, 10)
	for i := range out {
		for j := range buf {
			buf[j] = alphabet[randIndex(len(alphabet))]
		}
		out[i] = string(buf[:5]) + "-" + string(buf[5:])
	}
	return out
}

// randIndex returns a uniform random index below n (n < 256).
func randIndex(n int) int {
	limit := 256 - 256%n
	var b [1]byte
	for {
		_, _ = rand.Read(b[:])
		if int(b[0]) < limit {
			return int(b[0]) % n
		}
	}
}

// HashRecoveryCode normalizes a typed code and hashes it for storage. The codes
// are random enough that a fast hash is fine.
func HashRecoveryCode(code string) []byte {
	code = strings.ToLower(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(code)))
	sum := sha256.Sum256([]byte(code))
	return sum[:]
}
