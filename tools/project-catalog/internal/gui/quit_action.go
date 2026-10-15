package gui

import (
	"context"
	"sync/atomic"
	"time"
)

// Wait for the worker's terminal state, which follows its final checkpoint
// flush. Never block the UI thread or pretend to preempt an active OS call.
func newQuitAction(ctx context.Context, cancelWorker func(), running func() bool, dispatch func(func()), quit func()) func() {
	var pending atomic.Bool
	return func() {
		if ctx.Err() != nil || !pending.CompareAndSwap(false, true) {
			return
		}
		cancelWorker()
		finish := func() {
			if ctx.Err() == nil {
				quit()
			}
		}
		if !running() {
			dispatch(finish)
			return
		}
		go func() {
			ticker := time.NewTicker(25 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if !running() {
						dispatch(finish)
						return
					}
				}
			}
		}()
	}
}

// Native menus may invoke actions outside Fyne's event queue. Recheck lifecycle
// inside the dispatched callback as a window can close while it is queued.
func dispatchWhileAlive(ctx context.Context, dispatch func(func()), action func()) {
	if ctx.Err() != nil {
		return
	}
	dispatch(func() {
		if ctx.Err() == nil {
			action()
		}
	})
}
