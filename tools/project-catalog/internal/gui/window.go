//go:build cgo

package gui

import (
	"context"
	"scan-projects/internal/controller"
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
	defer func() { cleanupAccessibility(); cancel() }()
	a.Lifecycle().SetOnStopped(func() { cleanupAccessibility(); cancel(); ctrl.Cancel() })
	w := a.NewWindow("Scanner")
	w.SetOnClosed(func() { cleanupAccessibility(); cancel(); ctrl.Cancel() })
	w.Resize(fyne.NewSize(1200, 720))

	refreshActions := func() {}
	startBtn := widget.NewButton("Start", func() {
		if err := ctrl.Start(false); err != nil {
			dialog.ShowError(err, w)
		}
		refreshActions()
	})
	startResBtn := widget.NewButton("Start (Resume)", func() {
		if err := ctrl.Start(true); err != nil {
			dialog.ShowError(err, w)
		}
		refreshActions()
	})
	pauseBtn := widget.NewButton("Pause", func() { ctrl.Pause(); refreshActions() })
	resumeBtn := widget.NewButton("Resume", func() { ctrl.Resume(); refreshActions() })
	cancelBtn := widget.NewButton("Cancel", func() { ctrl.Cancel(); refreshActions() })

	addFolderBtn := widget.NewButton("Add Folder", func() { ShowFolderPicker(ctx, w, ctrl, nil) })
	profiles := NewProfilesSelect(ctx, w, ctrl)
	settingsBtn := widget.NewButton("Settings", func() {
		ShowConfigEditor(ctx, w, ctrl)
	})

	top := container.NewHBox(
		startBtn, startResBtn, pauseBtn, resumeBtn, cancelBtn,
		widget.NewSeparator(),
		addFolderBtn, widget.NewLabel("Profile:"),
		profiles,
		settingsBtn,
	)

	details := widget.NewLabel("Select a folder")
	details.Wrapping = fyne.TextWrapWord

	tree := NewFolderTree(ctx, ctrl, details)
	treePanel := container.NewBorder(widget.NewLabel("Folders: arrows navigate; Space selects"), nil, nil, nil, tree)
	split := container.NewHSplit(treePanel, container.NewVScroll(details))
	split.Offset = 0.42

	status := NewProgressPanel(ctx, ctrl)
	shortcutHelp := widget.NewLabel("Ctrl/Cmd: O Add Folder · S Start · Shift+S Resume Scan · P Pause · R Resume · K Cancel · B Profiles · L Folders · , Settings")
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
		setEnabled := func(button *widget.Button, enabled bool) {
			if enabled {
				button.Enable()
			} else {
				button.Disable()
			}
		}
		setEnabled(startBtn, !running)
		setEnabled(startResBtn, !running)
		setEnabled(pauseBtn, running && !paused && !canceling)
		setEnabled(resumeBtn, running && paused && !canceling)
		setEnabled(cancelBtn, running && !canceling)
		setEnabled(addFolderBtn, !running)
		if running {
			profiles.Disable()
		} else {
			profiles.Enable()
		}
	}
	refreshActions()
	startUIPoller(ctx, 200*time.Millisecond, fyne.Do, refreshActions)
	w.Canvas().Focus(startBtn)

	cleanupAccessibility = attachWindowAccessibility(ctx, w)
	SetupTray(a, w, ctrl)
	SetupCloseToTray(w)

	w.ShowAndRun()
}
