package gui

import (
	"context"
	"sync/atomic"
	"time"
)

// startUIPoller coalesces refreshes onto the UI dispatcher. The lifecycle owner
// cancels ctx on actual window/app shutdown, not while hiding to the tray.
// A queued refresh checks cancellation again before touching widgets.
func startUIPoller(ctx context.Context, interval time.Duration, dispatch func(func()), refresh func()) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var pending atomic.Bool
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if ctx.Err() != nil {
					return
				}
				if !pending.CompareAndSwap(false, true) {
					continue
				}
				dispatch(func() {
					defer pending.Store(false)
					if ctx.Err() == nil {
						refresh()
					}
				})
			}
		}
	}()
	return done
}
