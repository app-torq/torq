package engine

import "testing"

func i64(v int64) *int64 { return &v }

func TestAddMonthsClamping(t *testing.T) {
	cases := []struct {
		from string
		n    int
		want string
	}{
		{"2026-01-31", 1, "2026-02-28"},
		{"2028-01-31", 1, "2028-02-29"},
		{"2026-08-31", 6, "2027-02-28"},
		{"2026-03-15", 12, "2027-03-15"},
		{"2024-11-10", 24, "2026-11-10"},
		{"2026-12-31", 2, "2027-02-28"},
	}
	for _, c := range cases {
		if got := AddMonths(MustDate(c.from), c.n).String(); got != c.want {
			t.Errorf("AddMonths(%s, %d) = %s, want %s", c.from, c.n, got, c.want)
		}
	}
}

func TestNextDue(t *testing.T) {
	b := Baseline{Date: MustDate("2026-10-15"), Km: 215000}
	d := NextDue(b, Interval{Km: i64(10000)})
	if d.Km == nil || *d.Km != 225000 || d.Date != nil {
		t.Fatalf("km-only: %+v", d)
	}
	d = NextDue(b, Interval{Months: i64(12)})
	if d.Km != nil || d.Date.String() != "2027-10-15" {
		t.Fatalf("months-only: %+v", d)
	}
	d = NextDue(b, Interval{Days: i64(30)})
	if d.Date.String() != "2026-11-14" {
		t.Fatalf("days-only: %v", d.Date)
	}
	d = NextDue(b, Interval{Km: i64(10000), Months: i64(12)})
	if *d.Km != 225000 || d.Date.String() != "2027-10-15" {
		t.Fatalf("composite: %+v", d)
	}
}

func TestNextDueUnknownBaseline(t *testing.T) {
	b := Baseline{Date: MustDate("2026-10-15"), Km: 215000, Unknown: true}
	d := NextDue(b, Interval{Km: i64(10000), Months: i64(24)})
	if *d.Km != 215000 || d.Date.String() != "2026-10-15" {
		t.Fatalf("composite: %+v", d)
	}
	d = NextDue(b, Interval{Days: i64(30)})
	if d.Km != nil || d.Date.String() != "2026-10-15" {
		t.Fatalf("days-only: %+v", d)
	}
	st := Evaluate(NextDue(b, Interval{Months: i64(24)}), Lead{Days: i64(30)}, 215000, MustDate("2026-10-15"))
	if st.State != Overdue || st.Trigger != TriggerTime || *st.RemainingDays != 0 {
		t.Fatalf("evaluate: %+v", st)
	}
}

func TestEvaluateKmBoundaries(t *testing.T) {
	due := Due{Km: i64(225000)}
	lead := Lead{Km: i64(1000)}
	today := MustDate("2026-10-30")
	cases := []struct {
		km        int64
		state     string
		remaining int64
	}{
		{223999, Upcoming, 1001},
		{224000, DueSoon, 1000},
		{224100, DueSoon, 900},
		{224999, DueSoon, 1},
		{225000, Overdue, 0},
		{225350, Overdue, -350},
	}
	for _, c := range cases {
		st := Evaluate(due, lead, c.km, today)
		if st.State != c.state || *st.RemainingKm != c.remaining {
			t.Errorf("km %d: got %s/%d, want %s/%d", c.km, st.State, *st.RemainingKm, c.state, c.remaining)
		}
		if c.state != Upcoming && st.Trigger != TriggerKm {
			t.Errorf("km %d: trigger %q", c.km, st.Trigger)
		}
	}
}

func TestEvaluateTimeBoundaries(t *testing.T) {
	dueDate := MustDate("2026-11-10")
	due := Due{Date: &dueDate}
	lead := Lead{Days: i64(30)}
	cases := []struct {
		today string
		state string
		days  int64
	}{
		{"2026-10-10", Upcoming, 31},
		{"2026-10-11", DueSoon, 30},
		{"2026-11-09", DueSoon, 1},
		{"2026-11-10", Overdue, 0},
		{"2026-12-10", Overdue, -30},
	}
	for _, c := range cases {
		st := Evaluate(due, lead, 0, MustDate(c.today))
		if st.State != c.state || *st.RemainingDays != c.days {
			t.Errorf("%s: got %s/%d, want %s/%d", c.today, st.State, *st.RemainingDays, c.state, c.days)
		}
	}
}

func TestEvaluateCompositeMostUrgentWins(t *testing.T) {
	dueDate := MustDate("2027-10-15")
	due := Due{Km: i64(225000), Date: &dueDate}
	lead := Lead{Km: i64(1000), Days: i64(30)}

	// Km overdue, time far away → overdue by km.
	st := Evaluate(due, lead, 225100, MustDate("2026-11-01"))
	if st.State != Overdue || st.Trigger != TriggerKm {
		t.Fatalf("got %+v", st)
	}
	// Km fine, time overdue → overdue by time.
	st = Evaluate(due, lead, 216000, MustDate("2027-10-20"))
	if st.State != Overdue || st.Trigger != TriggerTime {
		t.Fatalf("got %+v", st)
	}
	// Km due soon, time overdue → overdue by time.
	st = Evaluate(due, lead, 224500, MustDate("2027-10-15"))
	if st.State != Overdue || st.Trigger != TriggerTime {
		t.Fatalf("got %+v", st)
	}
	// Neither → upcoming without trigger.
	st = Evaluate(due, lead, 216000, MustDate("2026-11-01"))
	if st.State != Upcoming || st.Trigger != "" {
		t.Fatalf("got %+v", st)
	}
}

func TestNilLeadIsZero(t *testing.T) {
	st := Evaluate(Due{Km: i64(1000)}, Lead{}, 999, MustDate("2026-01-01"))
	if st.State != Upcoming {
		t.Fatalf("nil lead must not create a due-soon window, got %s", st.State)
	}
}

// TestHoursDimension covers reminders by operating hours (in tenths): every
// 250 h or 12 months with a 25 h / 30 day lead, last done at 1,262 h.
func TestHoursDimension(t *testing.T) {
	b := Baseline{Date: MustDate("2026-10-15"), HoursTenths: 12620}
	iv := Interval{HoursTenths: i64(2500), Months: i64(12)}
	due := NextDue(b, iv)
	if due.HoursTenths == nil || *due.HoursTenths != 15120 || due.Date.String() != "2027-10-15" || due.Km != nil {
		t.Fatalf("due %+v", due)
	}
	lead := Lead{HoursTenths: i64(250), Days: i64(30)}
	today := MustDate("2026-11-01")
	for _, c := range []struct {
		hours        int64
		today        Date
		state, trig  string
		remainTenths int64
	}{
		{14869, today, Upcoming, "", 251},
		{14870, today, DueSoon, TriggerHours, 250},
		{15119, today, DueSoon, TriggerHours, 1},
		{15120, today, Overdue, TriggerHours, 0},
		{15500, today, Overdue, TriggerHours, -380},
		// Time comes first: hours far from due, date past due.
		{13000, MustDate("2027-10-15"), Overdue, TriggerTime, 2120},
		// Time due soon, hours overdue: the more urgent state wins.
		{15200, MustDate("2027-09-20"), Overdue, TriggerHours, -80},
	} {
		st := EvaluateWithHours(due, lead, 0, c.hours, c.today)
		if st.State != c.state || st.Trigger != c.trig || st.RemainingHoursTenths == nil || *st.RemainingHoursTenths != c.remainTenths {
			t.Errorf("hours %d on %s: %+v (remaining %v)", c.hours, c.today, st, st.RemainingHoursTenths)
		}
	}

	// Ties between equally urgent dimensions go to km, then hours, then time.
	all := Due{Km: i64(1000), HoursTenths: i64(100), Date: ptrDate(MustDate("2026-11-01"))}
	if st := EvaluateWithHours(all, Lead{}, 1000, 100, MustDate("2026-11-01")); st.Trigger != TriggerKm {
		t.Fatalf("km tie %+v", st)
	}
	if st := EvaluateWithHours(all, Lead{}, 999, 100, MustDate("2026-11-01")); st.Trigger != TriggerHours {
		t.Fatalf("hours tie %+v", st)
	}

	// Never done: due at the start reading right away; a nil lead is zero.
	never := NextDue(Baseline{Date: MustDate("2026-11-01"), HoursTenths: 12504, Unknown: true}, Interval{HoursTenths: i64(2500)})
	if st := EvaluateWithHours(never, Lead{}, 0, 12504, MustDate("2026-11-01")); st.State != Overdue || st.Trigger != TriggerHours {
		t.Fatalf("never done %+v", st)
	}
}

func ptrDate(d Date) *Date { return &d }
