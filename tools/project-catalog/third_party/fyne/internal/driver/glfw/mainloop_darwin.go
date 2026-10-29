//go:build darwin && !test_web_driver

package glfw

import (
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
)

// Keep Cocoa's run loop servicing native requests while idle, rather than
// sleeping in a Go select until the next frame. AX requests are synchronous IPC;
// deferring each request to a frame makes serial tree inspection take seconds.
// The frame ticker still bounds drawing to 60 Hz. The finite native wait also
// bounds how long queued Go work and shutdown can wait without a native event.
func nextMainEvent(done <-chan struct{}, functions <-chan funcData, frames <-chan time.Time) (mainEvent, funcData) {
	for {
		select {
		case <-done:
			return mainStop, funcData{}
		case f := <-functions:
			return mainFunction, f
		case <-frames:
			return mainFrame, funcData{}
		default:
			glfw.WaitEventsTimeout(1.0 / 60)
		}
	}
}
