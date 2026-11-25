package mfa

import (
	"testing"
	"time"
)

func TestStore(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	s := NewStore[int](func() time.Time { return now })
	id := s.Put(42)
	if v, ok := s.Get(id); !ok || v != 42 {
		t.Fatalf("get %d %v", v, ok)
	}
	for want := ChallengeAttempts - 1; want > 0; want-- {
		if left := s.Fail(id); left != want {
			t.Fatalf("left %d, want %d", left, want)
		}
	}
	if s.Fail(id) != 0 {
		t.Fatal("still attempts after the last")
	}
	if _, ok := s.Get(id); ok {
		t.Fatal("usable after too many failures")
	}

	id = s.Put(7)
	if _, ok := s.Take(id); !ok {
		t.Fatal("take")
	}
	if _, ok := s.Take(id); ok {
		t.Fatal("taken twice")
	}

	id = s.Put(9)
	now = now.Add(ChallengeTTL)
	if _, ok := s.Get(id); ok {
		t.Fatal("usable after it expired")
	}
	if a, b := s.Put(1), s.Put(1); a == b {
		t.Fatal("ids repeat")
	}
}
