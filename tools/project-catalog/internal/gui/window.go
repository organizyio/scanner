//go:build cgo

package gui

import (
	"scan-projects/internal/controller"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func Run(a fyne.App, ctrl *controller.Controller) {
	w := a.NewWindow("Scanner")
	w.Resize(fyne.NewSize(1200, 720))

	startBtn := widget.NewButton("Start", func() {
		if err := ctrl.Start(false); err != nil {
			dialog.ShowError(err, w)
		}
	})
	startResBtn := widget.NewButton("Start (Resume)", func() {
		if err := ctrl.Start(true); err != nil {
			dialog.ShowError(err, w)
		}
	})
	pauseBtn := widget.NewButton("Pause", func() { ctrl.Pause() })
	resumeBtn := widget.NewButton("Resume", func() { ctrl.Resume() })
	cancelBtn := widget.NewButton("Cancel", func() { ctrl.Cancel() })

	profiles := NewProfilesSelect(ctrl)
	settingsBtn := widget.NewButton("Settings", func() {
		ShowConfigEditor(w, ctrl)
	})

	top := container.NewHBox(
		startBtn, startResBtn, pauseBtn, resumeBtn, cancelBtn,
		widget.NewSeparator(),
		widget.NewLabel("Profile:"),
		profiles,
		settingsBtn,
	)

	details := widget.NewLabel("Select a folder")
	details.Wrapping = fyne.TextWrapWord

	tree := NewFolderTree(ctrl, details)
	split := container.NewHSplit(tree, container.NewVScroll(details))
	split.Offset = 0.42

	status := NewProgressPanel(ctrl)

	content := container.NewBorder(top, status, nil, nil, split)
	w.SetContent(content)

	SetupTray(a, w, ctrl)
	SetupCloseToTray(w)

	w.ShowAndRun()
}
