// Scanner adaptation, Copyright 2026 Arman Avanesyan.
// SPDX-License-Identifier: MIT
// See LICENSE-SCANNER in this module. Original Fyne code retains its BSD license.
package async

import "sync"

// ShutdownQueue seals submissions before draining cancellation acknowledgements.
// In contrast to inspecting len(Out()), Close also consumes accepted work that
// has not yet moved from the unbounded queue's input to its output.
type ShutdownQueue[T any] struct {
	mu     sync.RWMutex
	closed bool
	queue  *UnboundedChan[T]
}

func NewShutdownQueue[T any]() *ShutdownQueue[T] {
	return &ShutdownQueue[T]{queue: NewUnboundedChan[T]()}
}

// Submit returns false after sealing; the caller handles its post-shutdown path.
func (q *ShutdownQueue[T]) Submit(value T) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.closed {
		return false
	}
	q.queue.In() <- value
	return true
}

func (q *ShutdownQueue[T]) Out() <-chan T { return q.queue.Out() }

// Close is called by the queue's consumer after it stops normal execution.
// Cancellation runs outside the lock, and every accepted value is drained.
func (q *ShutdownQueue[T]) Close(cancel func(T)) {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.closed = true
	q.queue.Close()
	q.mu.Unlock()
	for value := range q.queue.Out() {
		cancel(value)
	}
}
