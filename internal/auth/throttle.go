package auth

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

const (
	maxLockout = 15 * time.Minute
	staleAfter = time.Hour
)

// Throttle slows down repeated failed logins per key, such as a username or a
// client IP (FR-006, research R8). State is in memory and resets on restart.
type Throttle struct {
	mu      sync.Mutex
	now     func() time.Time
	free    int
	entries map[string]*throttleEntry
	checks  int
}

type throttleEntry struct {
	failures     int
	blockedUntil time.Time
	lastFailure  time.Time
}

// NewThrottle creates a throttle using now as its clock that lets each key
// fail free times before blocking it.
func NewThrottle(now func() time.Time, free int) *Throttle {
	return &Throttle{now: now, free: free, entries: map[string]*throttleEntry{}}
}

// RetryAfter returns how long the caller must wait before another attempt is
// allowed for any of keys; zero means allowed.
func (t *Throttle) RetryAfter(keys ...string) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	t.checks++
	if t.checks%256 == 0 {
		for k, e := range t.entries {
			if now.Sub(e.lastFailure) > staleAfter && now.After(e.blockedUntil) {
				delete(t.entries, k)
			}
		}
	}
	var wait time.Duration
	for _, k := range keys {
		if e, ok := t.entries[k]; ok && now.Before(e.blockedUntil) {
			if d := e.blockedUntil.Sub(now); d > wait {
				wait = d
			}
		}
	}
	return wait
}

// Fail records a failed attempt for each key. Once a key has used up its free
// failures, it is blocked for 1s, 2s, 4s, ... up to 15 minutes.
func (t *Throttle) Fail(keys ...string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	for _, k := range keys {
		e, ok := t.entries[k]
		if !ok {
			e = &throttleEntry{}
			t.entries[k] = e
		}
		e.failures++
		e.lastFailure = now
		if e.failures >= t.free {
			delay := time.Second << min(e.failures-t.free, 20)
			if delay > maxLockout {
				delay = maxLockout
			}
			e.blockedUntil = now.Add(delay)
		}
	}
}

// Succeed clears the failure history of each key.
func (t *Throttle) Succeed(keys ...string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, k := range keys {
		delete(t.entries, k)
	}
}

// ClientIP returns the client address, honoring X-Forwarded-For only when the
// direct peer is a trusted proxy. It walks the header right to left and returns
// the first address that is not itself a trusted proxy.
func ClientIP(r *http.Request, trusted []netip.Prefix) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil || !isTrusted(peer, trusted) {
		return host
	}
	parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		addr, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
		if err != nil {
			break
		}
		if !isTrusted(addr, trusted) {
			return addr.String()
		}
	}
	return host
}

func isTrusted(a netip.Addr, trusted []netip.Prefix) bool {
	a = a.Unmap()
	for _, p := range trusted {
		if p.Contains(a) {
			return true
		}
	}
	return false
}
