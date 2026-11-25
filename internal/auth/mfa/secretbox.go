package mfa

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// KeyFile is the name of the key that encrypts authenticator secrets, in the
// data directory so a directory backup keeps everything (Constitution III).
const KeyFile = "secret.key"

// Box encrypts authenticator secrets with AES-256-GCM. The user id is bound
// as additional data, so a secret copied to another user doesn't decrypt.
type Box struct{ aead cipher.AEAD }

// OpenBox loads the key from dir, creating it on first use.
func OpenBox(dir string) (*Box, error) {
	path := filepath.Join(dir, KeyFile)
	key, err := os.ReadFile(path) //nolint:gosec // path is the configured data directory
	if errors.Is(err, os.ErrNotExist) {
		key = make([]byte, 32)
		_, _ = rand.Read(key)
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // see above
		if err != nil {
			return nil, err
		}
		if _, err := f.Write(key); err != nil {
			_ = f.Close()
			return nil, err
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("%s must hold 32 bytes, has %d", path, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

func ad(userID int64) []byte { return binary.BigEndian.AppendUint64(nil, uint64(userID)) } //nolint:gosec // ids are positive

// Seal encrypts plain for userID; the nonce is prepended.
func (b *Box) Seal(userID int64, plain []byte) []byte {
	nonce := make([]byte, b.aead.NonceSize())
	_, _ = rand.Read(nonce)
	return b.aead.Seal(nonce, nonce, plain, ad(userID))
}

// Open decrypts what Seal returned for userID.
func (b *Box) Open(userID int64, sealed []byte) ([]byte, error) {
	n := b.aead.NonceSize()
	if len(sealed) < n {
		return nil, errors.New("sealed secret too short")
	}
	return b.aead.Open(nil, sealed[:n], sealed[n:], ad(userID))
}
