//go:build !no_glfw && !mobile

package glfw

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Ready queue/shutdown/frame delivery must not enter a native wait. Native IPC
// while idle is covered separately by the serial AX qualification regression.
func TestNextMainEventReady(t *testing.T) {
	done := make(chan struct{})
	functions := make(chan funcData, 1)
	frames := make(chan time.Time, 1)
	called := false
	functions <- funcData{f: func() { called = true }}
	event, f := nextMainEvent(done, functions, frames)
	assert.Equal(t, mainFunction, event)
	f.f()
	assert.True(t, called)
	frames <- time.Now()
	event, _ = nextMainEvent(done, functions, frames)
	assert.Equal(t, mainFrame, event)
	close(done)
	event, _ = nextMainEvent(done, functions, frames)
	assert.Equal(t, mainStop, event)
}
