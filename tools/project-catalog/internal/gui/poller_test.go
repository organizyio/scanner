package gui

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestUIPollerDispatchesCoalescesAndStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queued := make(chan func(), 10)
	var refreshes atomic.Int64
	done := startUIPoller(ctx, time.Millisecond, func(f func()) { queued <- f }, func() { refreshes.Add(1) })
	var first func()
	select {
	case first = <-queued:
	case <-time.After(time.Second):
		t.Fatal("no queued UI update")
	}
	if refreshes.Load() != 0 {
		t.Fatal("refresh ran outside dispatcher")
	}
	time.Sleep(10 * time.Millisecond)
	if len(queued) != 0 {
		t.Fatal("duplicate pending refresh queued")
	}
	first()
	if refreshes.Load() != 1 {
		t.Fatal("dispatched refresh missing")
	}
	var stale func()
	select {
	case stale = <-queued:
	case <-time.After(time.Second):
		t.Fatal("next refresh not queued")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("poller survived lifecycle cancellation")
	}
	stale()
	if refreshes.Load() != 1 {
		t.Fatal("queued update ran after lifecycle closed")
	}
	if len(queued) != 0 {
		t.Fatal("unexpected duplicate update")
	}
}

func TestUIPollerAlreadyClosedDoesNotRefresh(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var calls atomic.Int64
	done := startUIPoller(ctx, time.Millisecond, func(f func()) { calls.Add(1); f() }, func() { calls.Add(1) })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("closed poller did not stop")
	}
	if calls.Load() != 0 {
		t.Fatal("closed poller dispatched work")
	}
}
