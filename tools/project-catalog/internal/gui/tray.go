//go:build cgo

package gui

import (
	"context"
	"scan-projects/internal/controller"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
)

func SetupTray(a fyne.App, w fyne.Window, ctrl *controller.Controller) {
	var quitting atomic.Bool
	finishQuit := newQuitAction(context.Background(), ctrl.Cancel, ctrl.IsRunning, fyne.Do, a.Quit)
	quit := func() {
		if quitting.CompareAndSwap(false, true) {
			dispatchWhileAlive(context.Background(), fyne.Do, finishQuit)
		}
	}
	setupTray(context.Background(), a, w, ctrl, quit, func() bool { return !quitting.Load() })
}

func setupTray(ctx context.Context, a fyne.App, w fyne.Window, ctrl *controller.Controller, quit func(), canStart func() bool) {
	desk, ok := a.(desktop.App)
	if !ok {
		// Not a desktop platform (mobile/web), no tray support
		return
	}

	ui := func(action func()) func() { return func() { dispatchWhileAlive(ctx, fyne.Do, action) } }
	menu := fyne.NewMenu("Scanner",
		fyne.NewMenuItem("Show", ui(func() {
			w.Show()
			w.RequestFocus()
		})),
		fyne.NewMenuItem("Hide", ui(func() {
			w.Hide()
		})),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Start", ui(func() {
			if !canStart() {
				return
			}
			if err := ctrl.Start(false); err != nil {
				dialog.ShowError(err, w)
			}
		})),
		fyne.NewMenuItem("Pause", func() { ctrl.Pause() }),
		fyne.NewMenuItem("Resume", func() { ctrl.Resume() }),
		fyne.NewMenuItem("Cancel", func() { ctrl.Cancel() }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Exit", quit),
	)

	desk.SetSystemTrayMenu(menu)
}

func SetupCloseToTray(w fyne.Window) {
	w.SetCloseIntercept(func() {
		w.Hide()
	})
}
