// Package local implements username/password authentication with argon2id.
// Only internal/auth and internal/httpapi may import it (Constitution IV).
package local

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Parameters sized for low-memory single-board computers (research R5).
const (
	memoryKiB   = 19456
	iterations  = 2
	parallelism = 1
	saltLen     = 16
	keyLen      = 32
)

// MaxPasswordBytes is the longest password accepted. Longer input is never
// hashed, so it can't match any stored password.
const MaxPasswordBytes = 1024

var errMalformedHash = errors.New("malformed argon2id hash")

// hashSlots bounds concurrent argon2id runs: each one allocates memoryKiB, so
// a burst of logins would otherwise exhaust a small host's RAM.
var hashSlots = make(chan struct{}, max(runtime.NumCPU(), 2))

func idKey(password string, salt []byte, iterations, memory uint32, parallelism uint8, keyLen uint32) []byte {
	hashSlots <- struct{}{}
	defer func() { <-hashSlots }()
	return argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLen)
}

type params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	salt        []byte
	key         []byte
}

// Hash returns a PHC-formatted argon2id hash of password.
func Hash(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := idKey(password, salt, iterations, memoryKiB, parallelism, keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memoryKiB, iterations, parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify reports whether password matches hash, comparing in constant time.
func Verify(hash, password string) (bool, error) {
	p, err := decode(hash)
	if err != nil {
		return false, err
	}
	if len(password) > MaxPasswordBytes {
		return false, nil
	}
	key := idKey(password, p.salt, p.iterations, p.memory, p.parallelism, uint32(len(p.key))) //nolint:gosec // key length comes from our own 32-byte hashes
	return subtle.ConstantTimeCompare(key, p.key) == 1, nil
}

// NeedsRehash reports whether hash uses parameters other than the current ones.
func NeedsRehash(hash string) bool {
	p, err := decode(hash)
	if err != nil {
		return true
	}
	return p.memory != memoryKiB || p.iterations != iterations || p.parallelism != parallelism ||
		len(p.salt) != saltLen || len(p.key) != keyLen
}

func decode(hash string) (params, error) {
	parts := strings.Split(hash, "$")
	// "", "argon2id", "v=19", "m=..,t=..,p=..", salt, key
	if len(parts) != 6 || parts[1] != "argon2id" {
		return params{}, errMalformedHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return params{}, errMalformedHash
	}
	var p params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism); err != nil {
		return params{}, errMalformedHash
	}
	var err error
	if p.salt, err = base64.RawStdEncoding.DecodeString(parts[4]); err != nil {
		return params{}, errMalformedHash
	}
	if p.key, err = base64.RawStdEncoding.DecodeString(parts[5]); err != nil || len(p.key) == 0 {
		return params{}, errMalformedHash
	}
	return p, nil
}
