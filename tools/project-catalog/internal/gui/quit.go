//go:build cgo

package gui

import (
	"context"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// Quit is an application action, including while a modal is open. It is distinct
// from closing/hiding the main window to the tray.
func registerQuitShortcut(ctx context.Context, w fyne.Window, onQuit func()) {
	guarded := func() {
		if ctx.Err() == nil {
			onQuit()
		}
	}
	quitItem := fyne.NewMenuItem("Quit", guarded)
	quitItem.IsQuit = true
	w.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu("File", quitItem)))
	w.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyQ, Modifier: fyne.KeyModifierShortcutDefault}, func(fyne.Shortcut) {
		guarded()
	})
}
