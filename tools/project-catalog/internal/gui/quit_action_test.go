package gui

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestQuitWaitsForWorkerCheckpointAndCoalesces(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var running atomic.Bool
	running.Store(true)
	var cancels atomic.Int32
	queued := make(chan func(), 1)
	var quits int
	action := newQuitAction(ctx, func() { cancels.Add(1) }, running.Load, func(f func()) { queued <- f }, func() { quits++ })
	action()
	action()
	if cancels.Load() != 1 || quits != 0 {
		t.Fatal("quit must request cancellation once without exiting an active worker")
	}
	select {
	case <-queued:
		t.Fatal("quit scheduled before worker completed")
	default:
	}
	running.Store(false) // Controller clears running only after final checkpoint flush.
	select {
	case f := <-queued:
		f()
	case <-time.After(time.Second):
		t.Fatal("terminal worker did not schedule quit")
	}
	if quits != 1 {
		t.Fatalf("quits=%d", quits)
	}
}

func TestQueuedQuitDoesNotRunAfterLifecycleEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var running atomic.Bool
	running.Store(true)
	queued := make(chan func(), 1)
	quit := false
	action := newQuitAction(ctx, func() {}, running.Load, func(f func()) { queued <- f }, func() { quit = true })
	action()
	running.Store(false)
	select {
	case f := <-queued:
		cancel()
		f()
	case <-time.After(time.Second):
		t.Fatal("quit not queued")
	}
	if quit {
		t.Fatal("queued action outlived window lifecycle")
	}
}
