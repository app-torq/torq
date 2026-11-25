package httpapi

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
)

// softKey is a software authenticator holding one passkey, so the tests can
// register and use passkeys the way a phone or a security key would.
type softKey struct {
	t      *testing.T
	origin string
	rpID   string
	id     []byte
	key    *ecdsa.PrivateKey
	handle []byte // the user handle the server gave at registration
	count  uint32
	// verify sets the user-verified flag (PIN or biometrics).
	verify bool
}

func newSoftKey(t *testing.T, origin, rpID string) *softKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 16)
	_, _ = rand.Read(id)
	return &softKey{t: t, origin: origin, rpID: rpID, id: id, key: key, verify: true}
}

var b64 = base64.RawURLEncoding

func (k *softKey) flags(extra byte) byte {
	f := byte(0x01) | extra // user present
	if k.verify {
		f |= 0x04
	}
	return f
}

func (k *softKey) authData(flags byte, attested []byte) []byte {
	rp := sha256.Sum256([]byte(k.rpID))
	k.count++
	out := append([]byte{}, rp[:]...)
	out = append(out, flags)
	out = binary.BigEndian.AppendUint32(out, k.count)
	return append(out, attested...)
}

func (k *softKey) clientData(typ, challenge string) []byte {
	b, _ := json.Marshal(map[string]any{"type": typ, "challenge": challenge, "origin": k.origin, "crossOrigin": false})
	return b
}

// options reads the challenge (and the user handle at registration) from the
// server's options.
func optionChallenge(t *testing.T, options map[string]any) (challenge string, userHandle string) {
	t.Helper()
	pk, ok := options["publicKey"].(map[string]any)
	if !ok {
		t.Fatalf("options without publicKey: %v", options)
	}
	challenge, _ = pk["challenge"].(string)
	if u, ok := pk["user"].(map[string]any); ok {
		userHandle, _ = u["id"].(string)
	}
	return challenge, userHandle
}

// create answers registration options with a new credential.
func (k *softKey) create(options map[string]any) map[string]any {
	k.t.Helper()
	challenge, handle := optionChallenge(k.t, options)
	h, err := b64.DecodeString(handle)
	if err != nil {
		k.t.Fatal(err)
	}
	k.handle = h
	x, y := k.key.X.FillBytes(make([]byte, 32)), k.key.Y.FillBytes(make([]byte, 32))
	cose, err := webauthncbor.Marshal(map[int]any{1: 2, 3: -7, -1: 1, -2: x, -3: y})
	if err != nil {
		k.t.Fatal(err)
	}
	attested := make([]byte, 16) // AAGUID
	attested = binary.BigEndian.AppendUint16(attested, uint16(len(k.id)))
	attested = append(attested, k.id...)
	attested = append(attested, cose...)
	att, err := webauthncbor.Marshal(map[string]any{"fmt": "none", "attStmt": map[string]any{}, "authData": k.authData(k.flags(0x40), attested)})
	if err != nil {
		k.t.Fatal(err)
	}
	return map[string]any{
		"id": b64.EncodeToString(k.id), "rawId": b64.EncodeToString(k.id), "type": "public-key",
		"response": map[string]any{
			"clientDataJSON":    b64.EncodeToString(k.clientData("webauthn.create", challenge)),
			"attestationObject": b64.EncodeToString(att),
		},
	}
}

// get answers request options with a signed assertion.
func (k *softKey) get(options map[string]any) map[string]any {
	k.t.Helper()
	challenge, _ := optionChallenge(k.t, options)
	auth := k.authData(k.flags(0), nil)
	cd := k.clientData("webauthn.get", challenge)
	sum := sha256.Sum256(append(append([]byte{}, auth...), sha256Sum(cd)...))
	sig, err := ecdsa.SignASN1(rand.Reader, k.key, sum[:])
	if err != nil {
		k.t.Fatal(err)
	}
	return map[string]any{
		"id": b64.EncodeToString(k.id), "rawId": b64.EncodeToString(k.id), "type": "public-key",
		"response": map[string]any{
			"clientDataJSON":    b64.EncodeToString(cd),
			"authenticatorData": b64.EncodeToString(auth),
			"signature":         b64.EncodeToString(sig),
			"userHandle":        b64.EncodeToString(k.handle),
		},
	}
}

func sha256Sum(b []byte) []byte {
	s := sha256.Sum256(b)
	return s[:]
}
