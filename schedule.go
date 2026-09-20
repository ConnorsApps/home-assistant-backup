package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
)

// runSchedule runs job at every tick until ctx is cancelled. A failed job
// doesn't stop the schedule, runs never overlap, and a run that overshoots a
// tick skips it.
func runSchedule(ctx context.Context, sched cron.Schedule, job func(context.Context) error) {
	for {
		next := sched.Next(time.Now())
		if next.IsZero() {
			slog.Warn("Schedule has no further runs")
			return
		}
		slog.Info("Next backup scheduled", "at", next)

		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		_ = job(ctx) // logs its own failures
		if ctx.Err() != nil {
			return
		}
	}
}
