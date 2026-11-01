// Package engine computes reminder due points and states. It is pure: no
// database, no clock (research R9).
package engine

import "time"

// State values (data-model §6).
const (
	Upcoming = "upcoming"
	DueSoon  = "due_soon"
	Overdue  = "overdue"
)

// Trigger values: which dimension produced the state.
const (
	TriggerKm   = "km"
	TriggerTime = "time"
)

const dateLayout = "2006-01-02"

// Date is a calendar date (no time of day, no zone).
type Date struct{ t time.Time }

// ParseDate parses YYYY-MM-DD.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return Date{}, err
	}
	return Date{t}, nil
}

// MustDate parses s or panics; for tests and constants.
func MustDate(s string) Date {
	d, err := ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

func (d Date) String() string { return d.t.Format(dateLayout) }

// AddDays returns d shifted by n days.
func (d Date) AddDays(n int) Date { return Date{d.t.AddDate(0, 0, n)} }

// DaysUntil returns the number of days from d to other (negative if other is earlier).
func (d Date) DaysUntil(other Date) int { return int(other.t.Sub(d.t).Hours() / 24) }

// Before reports whether d is strictly before other.
func (d Date) Before(other Date) bool { return d.t.Before(other.t) }

// AddMonths adds n months, clamping to the last day of shorter months
// (Jan 31 + 1 month = Feb 28/29).
func AddMonths(d Date, n int) Date {
	y, m, day := d.t.Date()
	first := time.Date(y, m, 1, 0, 0, 0, 0, time.UTC).AddDate(0, n, 0)
	last := first.AddDate(0, 1, -1).Day()
	if day > last {
		day = last
	}
	return Date{time.Date(first.Year(), first.Month(), day, 0, 0, 0, 0, time.UTC)}
}

// Interval is how often a reminder repeats. Months and Days are mutually exclusive.
type Interval struct {
	Km     *int64
	Months *int64
	Days   *int64
}

// HasTime reports whether the interval has a time component.
func (i Interval) HasTime() bool { return i.Months != nil || i.Days != nil }

// LengthDaysLowerBound is the shortest possible time interval in days (months × 28).
func (i Interval) LengthDaysLowerBound() int64 {
	switch {
	case i.Days != nil:
		return *i.Days
	case i.Months != nil:
		return *i.Months * 28
	default:
		return 0
	}
}

// Lead is the "due soon" threshold before the due point. Nil means zero.
type Lead struct {
	Km   *int64
	Days *int64
}

// Baseline is the last time the work was done (or the manual start point).
type Baseline struct {
	Date Date
	Km   int64
}

// Due is the next due point; a nil field means that dimension is not tracked.
type Due struct {
	Km   *int64
	Date *Date
}

// NextDue adds the interval to the baseline.
func NextDue(b Baseline, iv Interval) Due {
	var due Due
	if iv.Km != nil {
		km := b.Km + *iv.Km
		due.Km = &km
	}
	switch {
	case iv.Months != nil:
		d := AddMonths(b.Date, int(*iv.Months))
		due.Date = &d
	case iv.Days != nil:
		d := b.Date.AddDays(int(*iv.Days))
		due.Date = &d
	}
	return due
}

// Status is the evaluated state with the remaining distance/time to the due point.
// Remaining values are negative when overdue.
type Status struct {
	State         string
	Trigger       string // "" when upcoming
	RemainingKm   *int64
	RemainingDays *int64
}

func rank(state string) int {
	switch state {
	case Overdue:
		return 2
	case DueSoon:
		return 1
	default:
		return 0
	}
}

// Rank orders states by urgency (upcoming < due_soon < overdue).
func Rank(state string) int { return rank(state) }

// Evaluate computes the state for the current odometer and date:
// overdue if any limit is reached, due_soon if within any lead threshold,
// otherwise upcoming (data-model §6).
func Evaluate(due Due, lead Lead, curKm int64, today Date) Status {
	var st Status
	kmState, timeState := Upcoming, Upcoming
	if due.Km != nil {
		remaining := *due.Km - curKm
		st.RemainingKm = &remaining
		leadKm := int64(0)
		if lead.Km != nil {
			leadKm = *lead.Km
		}
		switch {
		case curKm >= *due.Km:
			kmState = Overdue
		case curKm >= *due.Km-leadKm:
			kmState = DueSoon
		}
	}
	if due.Date != nil {
		remaining := int64(today.DaysUntil(*due.Date))
		st.RemainingDays = &remaining
		leadDays := 0
		if lead.Days != nil {
			leadDays = int(*lead.Days)
		}
		switch {
		case !today.Before(*due.Date):
			timeState = Overdue
		case !today.Before(due.Date.AddDays(-leadDays)):
			timeState = DueSoon
		}
	}
	switch {
	case rank(kmState) >= rank(timeState) && kmState != Upcoming:
		st.State, st.Trigger = kmState, TriggerKm
	case timeState != Upcoming:
		st.State, st.Trigger = timeState, TriggerTime
	default:
		st.State = Upcoming
	}
	return st
}
