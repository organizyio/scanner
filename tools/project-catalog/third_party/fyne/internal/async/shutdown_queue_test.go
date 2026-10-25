// Scanner adaptation, Copyright 2026 Arman Avanesyan.
// SPDX-License-Identifier: MIT
package async

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestShutdownDrainsInputBeforeOutputIsPublished(t *testing.T) {
	// Hold the underlying processor until shutdown starts. This deterministically
	// reproduces accepted input with an empty output, missed by len(Out()) drains.
	channel := &UnboundedChan[int]{in: make(chan int, 16), out: make(chan int, 16), close: make(chan struct{})}
	q := &ShutdownQueue[int]{queue: channel}
	if !q.Submit(42) || len(q.Out()) != 0 {
		t.Fatal("fixture must have accepted input without output")
	}
	finished := make(chan int, 1)
	go q.Close(func(value int) { finished <- value })
	deadline := time.Now().Add(time.Second)
	for q.mu.TryRLock() {
		q.mu.RUnlock()
		if time.Now().After(deadline) {
			t.Fatal("shutdown did not reach its close handoff")
		}
		runtime.Gosched()
	}
	go channel.processing()
	select {
	case value := <-finished:
		if value != 42 {
			t.Fatal(value)
		}
	case <-time.After(time.Second):
		t.Fatal("accepted callback was stranded during shutdown")
	}
}

func TestShutdownReleasesAllConcurrentWaiters(t *testing.T) {
	for round := 0; round < 20; round++ {
		q := NewShutdownQueue[chan struct{}]()
		var workers sync.WaitGroup
		var accepted, canceled atomic.Int32
		for i := 0; i < 256; i++ {
			workers.Add(1)
			go func() {
				defer workers.Done()
				done := make(chan struct{})
				if q.Submit(done) {
					accepted.Add(1)
					<-done
				}
			}()
		}
		q.Close(func(done chan struct{}) { canceled.Add(1); close(done) })
		finished := make(chan struct{})
		go func() { workers.Wait(); close(finished) }()
		select {
		case <-finished:
		case <-time.After(2 * time.Second):
			t.Fatal("synchronous main-queue waiter remained blocked")
		}
		if accepted.Load() != canceled.Load() || q.Submit(make(chan struct{})) {
			t.Fatal("sealing lost an acknowledgement or accepted late work")
		}
	}
}

func TestShutdownDrainsMoreThanOutputCapacity(t *testing.T) {
	q := NewShutdownQueue[int]()
	for i := 0; i < 1024; i++ {
		if !q.Submit(i) {
			t.Fatal("queue sealed too early")
		}
	}
	seen := make([]bool, 1024)
	q.Close(func(i int) {
		if seen[i] {
			t.Fatal("duplicate cancellation")
		}
		seen[i] = true
	})
	for i, canceled := range seen {
		if !canceled {
			t.Fatal("missing cancellation", i)
		}
	}
	q.Close(func(int) { t.Fatal("idempotent close delivered twice") })
}
