package mfa

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// ChallengeTTL is how long a sign-in challenge or a passkey ceremony lasts.
const ChallengeTTL = 5 * time.Minute

// ChallengeAttempts is how many wrong answers a sign-in challenge takes.
const ChallengeAttempts = 5

// Store keeps short-lived values by random id in memory: sign-in challenges
// and passkey ceremonies. A restart drops them; people then sign in again.
type Store[T any] struct {
	mu    sync.Mutex
	now   func() time.Time
	items map[string]*entry[T]
}

type entry[T any] struct {
	value    T
	expires  time.Time
	attempts int
}

// NewStore returns an empty store.
func NewStore[T any](now func() time.Time) *Store[T] {
	return &Store[T]{now: now, items: map[string]*entry[T]{}}
}

// Put stores v and returns its id.
func (s *Store[T]) Put(v T) string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	id := base64.RawURLEncoding.EncodeToString(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweep()
	s.items[id] = &entry[T]{value: v, expires: s.now().Add(ChallengeTTL), attempts: ChallengeAttempts}
	return id
}

// Get returns the value for id while it is valid.
func (s *Store[T]) Get(id string) (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweep()
	e, ok := s.items[id]
	if !ok {
		var zero T
		return zero, false
	}
	return e.value, true
}

// Take returns the value for id and removes it, so it can be used once.
func (s *Store[T]) Take(id string) (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweep()
	e, ok := s.items[id]
	delete(s.items, id)
	if !ok {
		var zero T
		return zero, false
	}
	return e.value, true
}

// Fail counts a wrong answer and returns how many are left; at zero the id is
// gone.
func (s *Store[T]) Fail(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[id]
	if !ok {
		return 0
	}
	e.attempts--
	if e.attempts <= 0 {
		delete(s.items, id)
		return 0
	}
	return e.attempts
}

func (s *Store[T]) sweep() {
	now := s.now()
	for id, e := range s.items {
		if !now.Before(e.expires) {
			delete(s.items, id)
		}
	}
}
