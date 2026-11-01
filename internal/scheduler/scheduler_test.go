package scheduler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunsAtStartupAndIsolatesFailures(t *testing.T) {
	var ok atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	s := &Scheduler{
		Interval: time.Hour,
		Log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Tasks: []Task{
			{Name: "fails", Run: func(context.Context) error { return errors.New("boom") }},
			{Name: "ok", Run: func(context.Context) error { ok.Add(1); cancel(); return nil }},
		},
	}
	done := make(chan struct{})
	go func() { s.Run(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop")
	}
	if ok.Load() != 1 {
		t.Fatalf("second task ran %d times, want 1 despite first failing", ok.Load())
	}
}
