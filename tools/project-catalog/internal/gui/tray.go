//go:build cgo

package gui

import (
	"scan-projects/internal/controller"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

func SetupTray(a fyne.App, w fyne.Window, ctrl *controller.Controller) {
	desk, ok := a.(desktop.App)
	if !ok {
		// Not a desktop platform (mobile/web), no tray support
		return
	}

	menu := fyne.NewMenu("Scanner",
		fyne.NewMenuItem("Show", func() {
			w.Show()
			w.RequestFocus()
		}),
		fyne.NewMenuItem("Hide", func() {
			w.Hide()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Start", func() { _ = ctrl.Start(false) }),
		fyne.NewMenuItem("Pause", func() { ctrl.Pause() }),
		fyne.NewMenuItem("Resume", func() { ctrl.Resume() }),
		fyne.NewMenuItem("Cancel", func() { ctrl.Cancel() }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Exit", func() {
			ctrl.Cancel()
			a.Quit()
		}),
	)

	desk.SetSystemTrayMenu(menu)
}

func SetupCloseToTray(w fyne.Window) {
	w.SetCloseIntercept(func() {
		w.Hide()
	})
}
