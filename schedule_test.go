package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// everyTick makes tests run in milliseconds.
type everyTick struct{ d time.Duration }

func (e everyTick) Next(t time.Time) time.Time { return t.Add(e.d) }

func TestRunScheduleKeepsGoingAfterFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		runSchedule(ctx, everyTick{5 * time.Millisecond}, func(context.Context) error {
			if calls.Add(1) == 3 {
				cancel()
			}
			return errors.New("boom")
		})
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("did not stop after cancel")
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("job ran %d times, want 3", got)
	}
}

func TestRunScheduleStopsWhileWaiting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var calls atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		runSchedule(ctx, everyTick{time.Hour}, func(context.Context) error {
			calls.Add(1)
			return nil
		})
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("did not stop while waiting")
	}
	if calls.Load() != 0 {
		t.Error("job ran after cancel")
	}
}
