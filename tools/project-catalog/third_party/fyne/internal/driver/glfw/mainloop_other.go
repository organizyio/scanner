//go:build !darwin || test_web_driver

package glfw

import "time"

// Other platforms retain the original blocking Go event selection.
func nextMainEvent(done <-chan struct{}, functions <-chan funcData, frames <-chan time.Time) (mainEvent, funcData) {
	select {
	case <-done:
		return mainStop, funcData{}
	case f := <-functions:
		return mainFunction, f
	case <-frames:
		return mainFrame, funcData{}
	}
}
