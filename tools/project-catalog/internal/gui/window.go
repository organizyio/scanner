//go:build cgo

package gui

import (
	"context"
	"scan-projects/internal/controller"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

func Run(a fyne.App, ctrl *controller.Controller) {
	ctx, cancel := context.WithCancel(context.Background())
	cleanupAccessibility := func() {}
	defer func() { cancel(); cleanupAccessibility() }()
	a.Lifecycle().SetOnStopped(func() { cancel(); cleanupAccessibility(); ctrl.Cancel() })
	w := a.NewWindow("Scanner")
	w.SetOnClosed(func() { cleanupAccessibility(); cancel(); ctrl.Cancel() })
	w.Resize(fyne.NewSize(1200, 720))

	refreshActions := func() {}
	var quitting atomic.Bool
	startBtn := widget.NewButton("Start", func() {
		if quitting.Load() {
			return
		}
		if err := ctrl.Start(false); err != nil {
			dialog.ShowError(err, w)
		}
		refreshActions()
	})
	startResBtn := widget.NewButton("Start (Resume)", func() {
		if quitting.Load() {
			return
		}
		if err := ctrl.Start(true); err != nil {
			dialog.ShowError(err, w)
		}
		refreshActions()
	})
	pauseBtn := widget.NewButton("Pause", func() { ctrl.Pause(); refreshActions() })
	resumeBtn := widget.NewButton("Resume", func() { ctrl.Resume(); refreshActions() })
	cancelBtn := widget.NewButton("Cancel", func() { ctrl.Cancel(); refreshActions() })

	addFolderBtn := widget.NewButton("Add Folder", func() {
		if !quitting.Load() {
			ShowFolderPicker(ctx, w, ctrl, nil)
		}
	})
	finishQuit := newQuitAction(ctx, ctrl.Cancel, ctrl.IsRunning, fyne.Do, func() {
		cleanupAccessibility()
		cancel()
		a.Quit()
	})
	var quitBtn *widget.Button
	quit := func() {
		if !quitting.CompareAndSwap(false, true) {
			return
		}
		dispatchWhileAlive(ctx, fyne.Do, func() {
			quitBtn.SetText("Quitting: waiting for scan checkpoint")
			quitBtn.Disable()
			refreshActions()
			finishQuit()
		})
	}
	quitBtn = widget.NewButton("Quit", quit)
	registerQuitShortcut(ctx, w, quit)
	profiles := NewProfilesSelect(ctx, w, ctrl)
	settingsBtn := widget.NewButton("Settings", func() {
		if !quitting.Load() {
			showConfigEditor(ctx, w, ctrl, quit)
		}
	})

	top := container.NewHBox(
		startBtn, startResBtn, pauseBtn, resumeBtn, cancelBtn,
		widget.NewSeparator(),
		addFolderBtn, widget.NewLabel("Profile:"),
		profiles,
		settingsBtn, quitBtn,
	)

	details := widget.NewLabel("Select a folder")
	details.Wrapping = fyne.TextWrapOff

	tree := NewFolderTree(ctx, ctrl, details)
	treePanel := container.NewBorder(widget.NewLabel("Folders: arrows navigate; Space selects"), nil, nil, nil, tree)
	split := container.NewHSplit(treePanel, container.NewScroll(details))
	split.Offset = 0.42

	status := NewProgressPanel(ctx, ctrl)
	shortcutHelp := widget.NewLabel("Ctrl/Cmd: O Add Folder · S Start · Shift+S Resume Scan · P Pause · R Resume · K Cancel · B Profiles · L Folders · , Settings · Q Quit")
	shortcutHelp.Wrapping = fyne.TextWrapWord

	content := container.NewBorder(top, container.NewVBox(shortcutHelp, status), nil, nil, split)
	w.SetContent(content)
	shortcuts := []struct {
		key    fyne.KeyName
		shift  bool
		action func()
	}{
		{fyne.KeyO, false, addFolderBtn.OnTapped}, {fyne.KeyS, false, startBtn.OnTapped}, {fyne.KeyS, true, startResBtn.OnTapped}, {fyne.KeyP, false, pauseBtn.OnTapped}, {fyne.KeyB, false, profiles.OnTapped}, {fyne.KeyR, false, resumeBtn.OnTapped}, {fyne.KeyK, false, cancelBtn.OnTapped}, {fyne.KeyL, false, func() { w.Canvas().Focus(tree) }}, {fyne.KeyComma, false, settingsBtn.OnTapped},
	}
	for _, binding := range shortcuts {
		binding := binding
		modifier := fyne.KeyModifierShortcutDefault
		if binding.shift {
			modifier |= fyne.KeyModifierShift
		}
		w.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: binding.key, Modifier: modifier}, func(fyne.Shortcut) {
			// A modal owns keyboard input; shortcuts must not invoke background controls.
			if w.Canvas().Overlays().Top() != nil {
				return
			}
			if binding.key == fyne.KeyS && ctrl.IsRunning() {
				return
			}
			if (binding.key == fyne.KeyO || binding.key == fyne.KeyB) && ctrl.IsRunning() {
				return
			}
			binding.action()
		})
	}
	refreshActions = func() {
		running, paused := ctrl.IsRunning(), ctrl.IsPaused()
		canceling := ctrl.Progress().State() == "canceling"
		stopping := quitting.Load()
		setEnabled := func(button *widget.Button, enabled bool) {
			if enabled {
				button.Enable()
			} else {
				button.Disable()
			}
		}
		setEnabled(startBtn, !running && !stopping)
		setEnabled(startResBtn, !running && !stopping)
		setEnabled(pauseBtn, running && !paused && !canceling && !stopping)
		setEnabled(resumeBtn, running && paused && !canceling && !stopping)
		setEnabled(cancelBtn, running && !canceling && !stopping)
		setEnabled(addFolderBtn, !running && !stopping)
		if running || stopping {
			profiles.Disable()
		} else {
			profiles.Enable()
		}
	}
	refreshActions()
	startUIPoller(ctx, 200*time.Millisecond, fyne.Do, refreshActions)
	w.Canvas().Focus(startBtn)

	cleanupAccessibility = attachWindowAccessibility(ctx, w)
	setupTray(ctx, a, w, ctrl, quit, func() bool { return !quitting.Load() })
	SetupCloseToTray(w)

	w.ShowAndRun()
}
