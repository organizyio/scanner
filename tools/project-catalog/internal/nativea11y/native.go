//go:build cgo

package nativea11y

/*
#cgo darwin CFLAGS: -fblocks
#cgo darwin LDFLAGS: ${SRCDIR}/../../native-a11y/target/release/libscanner_native_a11y.a -framework AppKit -framework Foundation -framework CoreGraphics -liconv
#cgo linux LDFLAGS: ${SRCDIR}/../../native-a11y/target/release/libscanner_native_a11y.a -ldl -lpthread -lm -lxcb
#cgo windows LDFLAGS: ${SRCDIR}/../../native-a11y/target/release/libscanner_native_a11y.a -lole32 -loleaut32 -luiautomationcore -luserenv -lntdll -lws2_32 -lbcrypt
#include <stdlib.h>
#include "native.h"
*/
import "C"
import (
	"encoding/json"
	"errors"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"sync"
	"time"
	"unsafe"
)

// nativeCalls is immutable after construction. Its narrow seam lets tests model
// an OS-owner wait without manufacturing native pointers or GUI permissions.
type nativeCalls struct {
	retain  func(unsafe.Pointer)
	release func(unsafe.Pointer)
	update  func(unsafe.Pointer, uintptr, []byte) int
	close   func(unsafe.Pointer, uintptr)
	poll    func(unsafe.Pointer, uintptr) []Action
}

func platformCalls() nativeCalls {
	return nativeCalls{
		retain:  func(p unsafe.Pointer) { C.scanner_platform_retain(p) },
		release: func(p unsafe.Pointer) { C.scanner_platform_release(p) },
		update: func(p unsafe.Pointer, w uintptr, b []byte) int {
			text := C.CString(string(b))
			defer C.free(unsafe.Pointer(text))
			return int(C.scanner_platform_update(p, text, C.uintptr_t(w)))
		},
		close: func(p unsafe.Pointer, w uintptr) { C.scanner_platform_close(p, C.uintptr_t(w)) },
		poll: func(p unsafe.Pointer, w uintptr) []Action {
			text := C.scanner_platform_poll(p, C.uintptr_t(w))
			var actions []Action
			if text != nil {
				_ = json.Unmarshal([]byte(C.GoString(text)), &actions)
				C.scanner_a11y_string_free(text)
			}
			return actions
		},
	}
}

type Adapter struct {
	native  nativeCalls
	mu      sync.Mutex
	ptr     unsafe.Pointer
	window  uintptr
	stop    chan struct{}
	handler func(Action)
	source  fyne.Window
}

func attach(window fyne.Window, handler func(Action)) (*Adapter, error) {
	native, ok := window.(driver.NativeWindow)
	if !ok {
		return nil, errors.New("window has no native accessibility context")
	}
	a := &Adapter{stop: make(chan struct{}), handler: handler, source: window, native: platformCalls()}
	native.RunNative(func(ctx any) {
		switch c := ctx.(type) {
		case driver.MacWindowContext:
			a.window = c.NSWindow
		case driver.WindowsWindowContext:
			a.window = c.HWND
		case driver.X11WindowContext:
			a.window = c.WindowHandle
		case driver.WaylandWindowContext:
			// Native Wayland coordinates and focus are not X11 handles.
			a.window = 0
		}
	})
	if a.window == 0 {
		return nil, errors.New("native window must be created before accessibility attaches")
	}
	a.ptr = C.scanner_platform_attach(C.uintptr_t(a.window))
	if a.ptr == nil {
		return nil, errors.New("native accessibility adapter could not attach")
	}
	go a.poll()
	return a, nil
}

// acquire pins only the native control block; it never waits for the OS owner
// while holding the Go mutex. Close can therefore run on that owner thread.
func (a *Adapter) acquire() (unsafe.Pointer, uintptr, fyne.Window, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ptr == nil {
		return nil, 0, nil, errors.New("accessibility adapter closed")
	}
	a.native.retain(a.ptr)
	return a.ptr, a.window, a.source, nil
}
func (a *Adapter) Update(snapshot Snapshot) error {
	ptr, window, source, err := a.acquire()
	if err != nil {
		return err
	}
	defer a.native.release(ptr)
	snapshot = scaledSnapshot(snapshot, float64(source.Canvas().Scale()))
	bytes, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	if a.native.update(ptr, window, bytes) != 0 {
		return errors.New("native accessibility rejected semantic snapshot")
	}
	return nil
}
func (a *Adapter) Close() {
	a.mu.Lock()
	ptr, window := a.ptr, a.window
	if ptr == nil {
		a.mu.Unlock()
		return
	}
	a.ptr = nil
	a.source = nil
	close(a.stop)
	a.mu.Unlock()
	a.native.close(ptr, window)
	a.native.release(ptr)
}
func (a *Adapter) poll() {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-a.stop:
			return
		case <-ticker.C:
			ptr, window, _, err := a.acquire()
			if err != nil {
				return
			}
			actions := a.native.poll(ptr, window)
			a.native.release(ptr)
			for _, action := range actions {
				action := action
				fyne.Do(func() {
					a.mu.Lock()
					active := a.ptr != nil
					handler := a.handler
					a.mu.Unlock()
					if active && handler != nil {
						handler(action)
					}
				})
			}
		}
	}
}
