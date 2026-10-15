//go:build cgo

package nativea11y

import (
	"fyne.io/fyne/v2/test"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

func TestUnavailableNativeContextFailsExplicitly(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	window := app.NewWindow("Synthetic")
	if adapter, err := Attach(window, func(Action) {}); err == nil {
		adapter.Close()
		t.Fatal("test canvas cannot provide native accessibility")
	}
}

func TestCloseDoesNotWaitForMutexHeldByOwnerDispatchedUpdate(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	window := app.NewWindow("Synthetic")
	entered := make(chan struct{})
	closed := make(chan struct{})
	finished := make(chan error, 1)
	var stop sync.Once
	var refs atomic.Int32
	refs.Store(1)
	adapter := &Adapter{ptr: unsafe.Pointer(new(byte)), source: window, stop: make(chan struct{})}
	adapter.native = nativeCalls{
		retain: func(unsafe.Pointer) { refs.Add(1) }, release: func(unsafe.Pointer) { refs.Add(-1) },
		update: func(unsafe.Pointer, uintptr, []byte) int { close(entered); <-closed; return -1 },
		close:  func(unsafe.Pointer, uintptr) { stop.Do(func() { close(closed) }) },
	}
	defer stop.Do(func() { close(closed) })
	go func() { finished <- adapter.Update(Snapshot{}) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("update did not reach native owner wait")
	}
	closeDone := make(chan struct{})
	go func() { adapter.Close(); close(closeDone) }()
	select {
	case <-closeDone:
	case <-time.After(3 * time.Second):
		t.Fatal("Close deadlocked on update's Go mutex")
	}
	if err := <-finished; err == nil {
		t.Fatal("pending update must observe native closure")
	}
	adapter.Close()
	if refs.Load() != 0 {
		t.Fatal("native control references were not released", refs.Load())
	}
}
