//go:build cgo

package gui

import (
	"fmt"
	"strings"

	"scan-projects/internal/controller"
	"scan-projects/internal/storage"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func NewProfilesSelect(ctrl *controller.Controller) *widget.Select {
	names := ctrl.Profiles().Names()
	sel := widget.NewSelect(names, func(v string) {
		ctrl.Profiles().SetActive(v)
	})
	sel.SetSelected(ctrl.Profiles().ActiveName())
	return sel
}

func ShowConfigEditor(parent fyne.Window, ctrl *controller.Controller) {
	p := ctrl.Profiles().Active()

	w := fyne.CurrentApp().NewWindow("Settings")
	w.Resize(fyne.NewSize(800, 620))

	// roots editor (checkbox table-like)
	rootsBox := container.NewVBox(widget.NewLabelWithStyle("Scan Roots", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	for i := range p.ScanRoots {
		idx := i
		check := widget.NewCheck("", func(v bool) {
			p.ScanRoots[idx].Enabled = v
			ctrl.Profiles().UpdateActive(p)
		})
		check.SetChecked(p.ScanRoots[i].Enabled)

		pathEntry := widget.NewEntry()
		pathEntry.SetText(p.ScanRoots[i].Path)
		pathEntry.OnChanged = func(s string) {
			p.ScanRoots[idx].Path = s
			ctrl.Profiles().UpdateActive(p)
		}

		rootsBox.Add(container.NewBorder(nil, nil, check, nil, pathEntry))
	}

	addRootBtn := widget.NewButton("+ Add Root", func() {
		p.ScanRoots = append(p.ScanRoots, storage.ScanRoot{Path: "", Enabled: true})
		ctrl.Profiles().UpdateActive(p)
		w.Close()
		ShowConfigEditor(parent, ctrl)
	})

	excludes := widget.NewMultiLineEntry()
	excludes.SetText(strings.Join(p.ExcludeDirs, "\n"))
	excludes.OnChanged = func(s string) {
		p.ExcludeDirs = splitLines(s)
		ctrl.Profiles().UpdateActive(p)
	}

	maxFile := widget.NewEntry()
	maxFile.SetText(fmt.Sprintf("%d", p.MaxFileSizeMB))
	maxFile.OnChanged = func(s string) {
		if v, err := parseInt64(s); err == nil {
			p.MaxFileSizeMB = v
			ctrl.Profiles().UpdateActive(p)
		}
	}

	hashCheck := widget.NewCheck("", func(v bool) {
		p.HashFiles = v
		ctrl.Profiles().UpdateActive(p)
		// live mid-scan apply
		ctrl.Runtime().HashFiles.Store(v)
	})
	hashCheck.SetChecked(p.HashFiles)

	hashMax := widget.NewEntry()
	hashMax.SetText(fmt.Sprintf("%d", p.HashMaxFileSizeMB))
	hashMax.OnChanged = func(s string) {
		if v, err := parseInt64(s); err == nil {
			p.HashMaxFileSizeMB = v
			ctrl.Profiles().UpdateActive(p)
			ctrl.Runtime().HashMaxSizeBytes.Store(v * 1024 * 1024)
		}
	}

	projCheck := widget.NewCheck("", func(v bool) {
		p.ProjectDetection = v
		ctrl.Profiles().UpdateActive(p)
		ctrl.Runtime().ProjectDetection.Store(v)
	})
	projCheck.SetChecked(p.ProjectDetection)

	cpCheck := widget.NewCheck("", func(v bool) {
		p.CheckpointEnabled = v
		ctrl.Profiles().UpdateActive(p)
	})
	cpCheck.SetChecked(p.CheckpointEnabled)

	cpPath := widget.NewEntry()
	cpPath.SetText(p.CheckpointPath)
	cpPath.OnChanged = func(s string) {
		p.CheckpointPath = s
		ctrl.Profiles().UpdateActive(p)
	}

	validateBtn := widget.NewButton("Validate", func() {
		errs := controller.ValidateProfile(p)
		if len(errs) > 0 {
			dialog.ShowError(fmt.Errorf(controller.JoinErrors(errs)), w)
			return
		}
		dialog.ShowInformation("OK", "Profile looks valid.", w)
	})

	rootsBox.Add(addRootBtn)

	form := widget.NewForm(
		widget.NewFormItem("Exclude Dirs (one per line)", excludes),
		widget.NewFormItem("Max File Size MB (0=none)", maxFile),
		widget.NewFormItem("Hash Files (live)", hashCheck),
		widget.NewFormItem("Hash Max File Size MB (live)", hashMax),
		widget.NewFormItem("Project Detection (live)", projCheck),
		widget.NewFormItem("Checkpoint Enabled", cpCheck),
		widget.NewFormItem("Checkpoint Path", cpPath),
	)

	content := container.NewVScroll(container.NewVBox(
		rootsBox,
		widget.NewSeparator(),
		form,
		validateBtn,
	))
	w.SetContent(content)
	w.Show()
}

func splitLines(s string) []string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func parseInt64(s string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &v)
	return v, err
}
