package vehicles

import (
	"context"
	"database/sql"
	"testing"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/store"
	"github.com/app-torq/torq/internal/testutil"
)

func newService(t *testing.T) (*Service, context.Context) {
	t.Helper()
	d := testutil.NewDB(t)
	return &Service{DB: d, Clock: testutil.FixedClock(t, "2026-10-30")}, context.Background()
}

func createS203(t *testing.T, s *Service, km int64) store.Vehicle {
	t.Helper()
	year := int64(2006)
	v, err := s.Create(context.Background(), Profile{Make: "Mercedes-Benz", Model: "C 220 CDI", ChassisCode: "S203", ModelYear: &year, EngineCode: "OM646"}, km, 0)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func conflictCode(t *testing.T, err error) string {
	t.Helper()
	e, ok := apperr.As(err)
	if !ok {
		t.Fatalf("expected apperr, got %v", err)
	}
	return e.Code
}

func TestQuickUpdateRules(t *testing.T) {
	s, ctx := newService(t)
	v := createS203(t, s, 214500)

	got, err := s.QuickUpdate(ctx, v.ID, QuickUpdateInput{OdometerKm: 215200})
	if err != nil || got.CurrentOdometerKm != 215200 {
		t.Fatalf("raise: %v %v", got.CurrentOdometerKm, err)
	}

	_, err = s.QuickUpdate(ctx, v.ID, QuickUpdateInput{OdometerKm: 21520})
	if code := conflictCode(t, err); code != "odometer.decrease_requires_confirmation" {
		t.Fatalf("code %q", code)
	}
	if e, _ := apperr.As(err); e.Params["current_km"] != int64(215200) {
		t.Fatalf("params %v", e.Params)
	}

	_, err = s.QuickUpdate(ctx, v.ID, QuickUpdateInput{OdometerKm: 330000})
	if code := conflictCode(t, err); code != "odometer.large_jump_requires_confirmation" {
		t.Fatalf("code %q", code)
	}
	if got, err = s.QuickUpdate(ctx, v.ID, QuickUpdateInput{OdometerKm: 330000, ConfirmLargeJump: true}); err != nil || got.CurrentOdometerKm != 330000 {
		t.Fatalf("confirmed jump: %v %v", got.CurrentOdometerKm, err)
	}
	if got, err = s.QuickUpdate(ctx, v.ID, QuickUpdateInput{OdometerKm: 215300, ConfirmDecrease: true}); err != nil || got.CurrentOdometerKm != 215300 {
		t.Fatalf("confirmed decrease: %v %v", got.CurrentOdometerKm, err)
	}

	readings, err := s.Readings(ctx, v.ID, "", "")
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]int{}
	for _, r := range readings {
		sources[r.Source]++
	}
	if sources[SourceInitial] != 1 || sources[SourceQuickUpdate] != 3 {
		t.Fatalf("sources %v", sources)
	}
}

func TestDoublingNeedsConfirmation(t *testing.T) {
	s, ctx := newService(t)
	v := createS203(t, s, 1000)
	if _, err := s.QuickUpdate(ctx, v.ID, QuickUpdateInput{OdometerKm: 2500}); conflictCode(t, err) != "odometer.large_jump_requires_confirmation" {
		t.Fatal("doubling must need confirmation")
	}
	zero := createS203(t, s, 0)
	if _, err := s.QuickUpdate(ctx, zero.ID, QuickUpdateInput{OdometerKm: 150}); err != nil {
		t.Fatalf("first km on a new vehicle must not need confirmation: %v", err)
	}
}

func TestQuickUpdateValidation(t *testing.T) {
	s, ctx := newService(t)
	v := createS203(t, s, 100)
	_, err := s.QuickUpdate(ctx, v.ID, QuickUpdateInput{OdometerKm: 200, ReadingDate: "2026-11-02"})
	if e, ok := apperr.As(err); !ok || e.Fields["reading_date"] != "validation.future_date" {
		t.Fatalf("got %v", err)
	}
}

func TestRecordReadingNeverLowers(t *testing.T) {
	s, ctx := newService(t)
	v := createS203(t, s, 215000)
	for _, tc := range []struct {
		km   int64
		want int64
	}{{205000, 215000}, {216000, 216000}, {210000, 216000}} {
		err := s.DB.WithTx(ctx, func(tx *sql.Tx) error {
			_, err := s.RecordReading(ctx, tx, v.ID, "2026-10-01", tc.km, SourceServiceLog)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		got, _ := s.Get(ctx, v.ID)
		if got.CurrentOdometerKm != tc.want {
			t.Fatalf("after %d: current %d, want %d", tc.km, got.CurrentOdometerKm, tc.want)
		}
	}
}

func TestAfterOdometerChangeHookRunsInTx(t *testing.T) {
	s, ctx := newService(t)
	v := createS203(t, s, 100)
	calls := 0
	s.AfterOdometerChange = func(_ context.Context, tx *sql.Tx, id int64) error {
		calls++
		var km int64
		if err := tx.QueryRowContext(ctx, "SELECT current_odometer_km FROM vehicles WHERE id = ?", id).Scan(&km); err != nil {
			return err
		}
		if km != 150 {
			t.Errorf("hook sees %d, want the new value 150", km)
		}
		return nil
	}
	if _, err := s.QuickUpdate(ctx, v.ID, QuickUpdateInput{OdometerKm: 150}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("hook calls %d", calls)
	}
}
