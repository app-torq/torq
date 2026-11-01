// Package scheduler runs periodic background work in-process (research R11).
package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Task is one unit of periodic work. Each task fails independently.
type Task struct {
	Name string
	Run  func(ctx context.Context) error
}

// Scheduler runs its tasks once at startup and then on every tick.
type Scheduler struct {
	Interval time.Duration
	Tasks    []Task
	Log      *slog.Logger
}

// Run blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	s.tick(ctx)
	t := time.NewTicker(s.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	for _, task := range s.Tasks {
		if ctx.Err() != nil {
			return
		}
		start := time.Now()
		if err := task.Run(ctx); err != nil {
			s.Log.ErrorContext(ctx, "scheduled task failed", "task", task.Name, "err", err)
			continue
		}
		s.Log.DebugContext(ctx, "scheduled task done", "task", task.Name, "duration_ms", time.Since(start).Milliseconds())
	}
}
