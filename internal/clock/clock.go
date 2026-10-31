// Package clock abstracts the current time so date-dependent logic is testable.
package clock

import "time"

// DateLayout is the storage format for calendar dates.
const DateLayout = "2006-01-02"

// Clock provides the current instant and local calendar date.
type Clock interface {
	Now() time.Time
	// Today returns the current local calendar date as YYYY-MM-DD.
	Today() string
}

// System is the real clock using the process time zone (TZ).
type System struct{}

func (System) Now() time.Time { return time.Now() }
func (System) Today() string  { return time.Now().Format(DateLayout) }

// Fixed is a deterministic clock for tests. Set At to move it.
type Fixed struct{ At time.Time }

func (f *Fixed) Now() time.Time          { return f.At }
func (f *Fixed) Today() string           { return f.At.Format(DateLayout) }
func (f *Fixed) Advance(d time.Duration) { f.At = f.At.Add(d) }

// Timestamp formats t as RFC 3339 UTC, the storage format for timestamps.
func Timestamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }
