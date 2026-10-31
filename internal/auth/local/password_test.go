package local

import (
	"strings"
	"testing"
)

func TestHashVerifyRoundTrip(t *testing.T) {
	h, err := Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("unexpected format %q", h)
	}
	ok, err := Verify(h, "correct horse battery")
	if err != nil || !ok {
		t.Fatalf("verify: %v %v", ok, err)
	}
	ok, err = Verify(h, "wrong password!")
	if err != nil || ok {
		t.Fatalf("wrong password verified: %v %v", ok, err)
	}
	if NeedsRehash(h) {
		t.Fatal("fresh hash should not need rehash")
	}
}

func TestNeedsRehash(t *testing.T) {
	old := "$argon2id$v=19$m=65536,t=3,p=2$c2FsdHNhbHRzYWx0c2FsdA$a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2U"
	if !NeedsRehash(old) {
		t.Fatal("different parameters must need rehash")
	}
	if !NeedsRehash("garbage") {
		t.Fatal("malformed hash must need rehash")
	}
	if _, err := Verify("garbage", "x"); err == nil {
		t.Fatal("malformed hash must error")
	}
}
