package notify

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/app-torq/torq/internal/clock"
	"github.com/app-torq/torq/internal/reminders"
	"github.com/app-torq/torq/internal/store"
	"github.com/app-torq/torq/internal/testutil"
	"github.com/app-torq/torq/internal/vehicles"
)

type sink struct {
	mu       sync.Mutex
	requests []*http.Request
	bodies   []string
	status   int
}

func (s *sink) handler(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.requests = append(s.requests, r)
	s.bodies = append(s.bodies, string(b))
	code := s.status
	s.mu.Unlock()
	w.WriteHeader(code)
	_, _ = w.Write([]byte(strings.Repeat("r", 3000)))
}

func (s *sink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

type fx struct {
	ctx  context.Context
	clk  *clock.Fixed
	n    *Service
	veh  *vehicles.Service
	rem  *reminders.Service
	vid  int64
	sink *sink
	srv  *httptest.Server
	q    *store.Queries
}

func setup(t *testing.T) *fx {
	t.Helper()
	d := testutil.NewDB(t)
	clk := testutil.FixedClock(t, "2026-10-30")
	f := &fx{ctx: context.Background(), clk: clk, sink: &sink{status: 200}, q: store.New(d.Writer)}
	f.srv = httptest.NewServer(http.HandlerFunc(f.sink.handler))
	t.Cleanup(f.srv.Close)
	f.n = New(d, clk, "https://torq.example", "test", slog.New(slog.NewTextHandler(io.Discard, nil)))
	f.veh = &vehicles.Service{DB: d, Clock: clk}
	f.rem = &reminders.Service{DB: d, Clock: clk, Vehicles: f.veh, OnTransitions: f.n.EnqueueTransitions}
	f.veh.AfterOdometerChange = f.rem.EvaluateVehicle
	v, err := f.veh.Create(f.ctx, vehicles.Profile{Make: "Mercedes-Benz", Model: "C 220 CDI"}, 200000, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.vid = v.ID
	return f
}

func (f *fx) webhook(t *testing.T, url string, in WebhookInput) Webhook {
	t.Helper()
	in.Name, in.URL = "test", url
	if in.Preset == "" {
		in.Preset = "generic_json"
	}
	wh, err := f.n.Create(f.ctx, in, 0)
	if err != nil {
		t.Fatal(err)
	}
	return wh
}

func (f *fx) reminder(t *testing.T) {
	t.Helper()
	km := int64(200000)
	every := int64(10000)
	if _, err := f.rem.Create(f.ctx, reminders.Input{VehicleID: f.vid, Title: "Oil", IntervalKm: &every, StartOdometerKm: &km}, 0); err != nil {
		t.Fatal(err)
	}
}

func (f *fx) odometer(t *testing.T, km int64) {
	t.Helper()
	if _, err := f.veh.QuickUpdate(f.ctx, f.vid, vehicles.QuickUpdateInput{OdometerKm: km, ConfirmDecrease: true, ConfirmLargeJump: true}); err != nil {
		t.Fatal(err)
	}
}

func (f *fx) deliveries(t *testing.T, webhookID int64) []store.WebhookDelivery {
	t.Helper()
	rows, err := f.q.ListDeliveries(f.ctx, store.ListDeliveriesParams{WebhookID: webhookID, Limit: 500})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestOneDeliveryPerTransition(t *testing.T) {
	f := setup(t)
	secret := "Bearer s3cret"
	sub := f.webhook(t, f.srv.URL, WebhookInput{Enabled: true, DueSoon: true, Overdue: true, Headers: []HeaderInput{{Name: "Authorization", Value: &secret}}})
	unsub := f.webhook(t, f.srv.URL, WebhookInput{Enabled: true, Overdue: true})
	disabled := f.webhook(t, f.srv.URL, WebhookInput{Enabled: false, DueSoon: true})
	f.reminder(t)

	f.odometer(t, 209100) // due soon
	for i := 0; i < 3; i++ {
		f.odometer(t, 209200+int64(i)*100)
		if err := f.rem.EvaluateAll(f.ctx); err != nil {
			t.Fatal(err)
		}
	}
	if got := len(f.deliveries(t, sub.ID)); got != 1 {
		t.Fatalf("subscribed endpoint deliveries %d, want 1", got)
	}
	if len(f.deliveries(t, unsub.ID)) != 0 || len(f.deliveries(t, disabled.ID)) != 0 {
		t.Fatal("unsubscribed or disabled endpoint got a delivery")
	}
	d := f.deliveries(t, sub.ID)[0]
	if strings.Contains(d.RequestHeadersJson, "s3cret") {
		t.Fatal("custom header value persisted in the delivery")
	}

	if err := f.n.ProcessDue(f.ctx); err != nil {
		t.Fatal(err)
	}
	if f.sink.count() != 1 || f.sink.requests[0].Header.Get("Authorization") != secret || f.sink.requests[0].Header.Get("User-Agent") != "torq/test" {
		t.Fatalf("sent %d, headers %v", f.sink.count(), f.sink.requests[0].Header)
	}
	if !strings.Contains(f.sink.bodies[0], `"event":"reminder_due_soon"`) {
		t.Fatalf("body %s", f.sink.bodies[0])
	}
	got := f.deliveries(t, sub.ID)[0]
	if got.Status != "succeeded" || len(got.ResponseExcerpt) != 2048 || got.LastStatusCode.Int64 != 200 {
		t.Fatalf("delivery %+v", got)
	}

	// Overdue, then back to upcoming and re-entering due soon notifies again.
	f.odometer(t, 210500)
	f.odometer(t, 205000)
	f.odometer(t, 209500)
	events := []string{}
	for _, d := range f.deliveries(t, sub.ID) {
		events = append(events, d.Event)
	}
	if len(events) != 3 {
		t.Fatalf("events %v", events)
	}
}

func TestRetriesAndFailure(t *testing.T) {
	f := setup(t)
	f.sink.status = 500
	wh := f.webhook(t, f.srv.URL, WebhookInput{Enabled: true, DueSoon: true})
	f.reminder(t)
	f.odometer(t, 209100)

	delays := []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute}
	for i := 0; i < 4; i++ {
		if err := f.n.ProcessDue(f.ctx); err != nil {
			t.Fatal(err)
		}
		d := f.deliveries(t, wh.ID)[0]
		if d.AttemptCount != int64(i+1) {
			t.Fatalf("attempt %d: count %d", i+1, d.AttemptCount)
		}
		if i < 3 {
			if d.Status != "pending" {
				t.Fatalf("attempt %d: status %s", i+1, d.Status)
			}
			// Not due yet: nothing is sent before the backoff elapses.
			_ = f.n.ProcessDue(f.ctx)
			if f.sink.count() != i+1 {
				t.Fatalf("sent before backoff elapsed")
			}
			f.clk.Advance(delays[i])
		} else if d.Status != "failed" || d.LastStatusCode.Int64 != 500 {
			t.Fatalf("final %+v", d)
		}
	}
}

func TestNonRetryable4xxAndRedirects(t *testing.T) {
	f := setup(t)
	f.sink.status = 404
	wh := f.webhook(t, f.srv.URL, WebhookInput{Enabled: true, DueSoon: true})
	f.reminder(t)
	f.odometer(t, 209100)
	_ = f.n.ProcessDue(f.ctx)
	if d := f.deliveries(t, wh.ID)[0]; d.Status != "failed" || d.AttemptCount != 1 {
		t.Fatalf("404 should fail immediately: %+v", d)
	}

	redirect := httptest.NewServer(http.RedirectHandler(f.srv.URL, http.StatusFound))
	defer redirect.Close()
	before := f.sink.count()
	res, err := f.n.SendTest(f.ctx, f.webhook(t, redirect.URL, WebhookInput{Enabled: true}).ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "failed" || *res.StatusCode != 302 || f.sink.count() != before {
		t.Fatalf("redirect followed: %+v", res)
	}

	f.sink.status = 429
	wh2 := f.webhook(t, f.srv.URL, WebhookInput{Enabled: true, Overdue: true})
	f.odometer(t, 211000)
	_ = f.n.ProcessDue(f.ctx)
	if d := f.deliveries(t, wh2.ID)[0]; d.Status != "pending" {
		t.Fatalf("429 should be retried: %+v", d)
	}
}

func TestUnreachableEndpointClassified(t *testing.T) {
	f := setup(t)
	res, err := f.n.SendTest(f.ctx, f.webhook(t, "http://127.0.0.1:1/hook", WebhookInput{Enabled: true}).ID)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "failed" || res.Error == nil || *res.Error != "connection_refused" {
		t.Fatalf("result %+v", res)
	}
}

func TestDigestsOncePerPeriod(t *testing.T) {
	f := setup(t)
	wh := f.webhook(t, f.srv.URL, WebhookInput{Enabled: true, WeeklyDigest: true, MonthlyDigest: true})
	// Created on Friday 2026-10-30: no digest for the slot before creation.
	if err := f.n.EnqueueDigests(f.ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.deliveries(t, wh.ID)) != 0 {
		t.Fatal("digest sent for a slot before the endpoint existed")
	}
	// Monday 2026-11-02 08:00 local: weekly slot; also past the monthly slot (1st, 08:00).
	f.clk.At = time.Date(2026, 11, 2, 8, 30, 0, 0, time.Local)
	for i := 0; i < 3; i++ {
		if err := f.n.EnqueueDigests(f.ctx); err != nil {
			t.Fatal(err)
		}
	}
	ds := f.deliveries(t, wh.ID)
	if len(ds) != 2 {
		t.Fatalf("deliveries %d, want weekly + monthly once", len(ds))
	}
	if !strings.Contains(string(ds[0].RequestBody), "Nothing due") && !strings.Contains(string(ds[1].RequestBody), "Nothing due") {
		t.Fatal("empty digest must say nothing is due")
	}
}

func TestSlots(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, loc) // Friday
	if got := lastWeeklySlot(now, 1, 8); !got.Equal(time.Date(2026, 10, 26, 8, 0, 0, 0, loc)) {
		t.Fatalf("weekly %v", got)
	}
	if got := lastWeeklySlot(now, 5, 13); !got.Equal(time.Date(2026, 10, 23, 13, 0, 0, 0, loc)) {
		t.Fatalf("weekly same day later hour %v", got)
	}
	if got := lastMonthlySlot(now, 1, 8); !got.Equal(time.Date(2026, 10, 1, 8, 0, 0, 0, loc)) {
		t.Fatalf("monthly %v", got)
	}
}

func TestRetentionKeeps100(t *testing.T) {
	f := setup(t)
	wh := f.webhook(t, f.srv.URL, WebhookInput{Enabled: true})
	for i := 0; i < 105; i++ {
		if _, err := f.n.SendTest(f.ctx, wh.ID); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(f.deliveries(t, wh.ID)); n != 100 {
		t.Fatalf("kept %d deliveries", n)
	}
}
