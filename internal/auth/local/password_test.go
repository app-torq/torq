package local

import (
	"strings"
	"testing"
	"time"
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

func TestVerifyRejectsOverlongPassword(t *testing.T) {
	long := strings.Repeat("a", MaxPasswordBytes+1)
	h, err := Hash(long)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := Verify(h, long); err != nil || ok {
		t.Fatalf("over-long password verified: %v %v", ok, err)
	}
}

func TestHashingIsBounded(t *testing.T) {
	h, err := Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	for range cap(hashSlots) {
		hashSlots <- struct{}{}
	}
	done := make(chan struct{})
	go func() {
		_, _ = Verify(h, "correct horse battery")
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("verify ran although every slot was taken")
	case <-time.After(50 * time.Millisecond):
	}
	for range cap(hashSlots) {
		<-hashSlots
	}
	<-done
}
