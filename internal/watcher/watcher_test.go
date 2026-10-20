package watcher

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type syncFunc func(context.Context) error

func (f syncFunc) Sync(ctx context.Context) error { return f(ctx) }

func runAsync(ctx context.Context, w *Watcher) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()
	return done
}

func TestRunSyncsUntilCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls atomic.Int32
	w := New(syncFunc(func(context.Context) error {
		if calls.Add(1) == 3 {
			cancel()
		}
		return errors.New("sync failed")
	}))
	w.interval, w.spread = time.Millisecond, 0

	select {
	case <-runAsync(ctx, w):
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancel")
	}

	if got := calls.Load(); got != 3 {
		t.Errorf("Sync calls = %d, want 3 (errors must not stop the loop)", got)
	}
}

func TestRunStopsBeforeFirstTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var calls atomic.Int32
	w := New(syncFunc(func(context.Context) error {
		calls.Add(1)
		return nil
	}))

	select {
	case <-runAsync(ctx, w):
	case <-time.After(time.Second):
		t.Fatal("Run did not return on cancelled context")
	}

	if got := calls.Load(); got != 0 {
		t.Errorf("Sync calls = %d, want 0", got)
	}
}

func TestWithJitter(t *testing.T) {
	base, spread := 5*time.Minute, 2*time.Minute

	for range 1000 {
		got := withJitter(base, spread)
		if got < base-spread || got >= base+spread {
			t.Fatalf("withJitter(%v, %v) = %v, want in [%v, %v)", base, spread, got, base-spread, base+spread)
		}
	}

	for _, spread := range []time.Duration{0, -time.Second} {
		if got := withJitter(base, spread); got != base {
			t.Errorf("withJitter(%v, %v) = %v, want %v", base, spread, got, base)
		}
	}
}
