//go:build cgo

package gui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"scan-projects/internal/controller"
	"scan-projects/internal/storage"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// NewProfilesSelect uses app-owned buttons so profile choices share the same
// keyboard and native accessibility actions instead of private popup internals.
func NewProfilesSelect(ctx context.Context, parent fyne.Window, ctrl *controller.Controller) *widget.Button {
	button := widget.NewButton("Profile: "+ctrl.Profiles().ActiveName(), nil)
	button.OnTapped = func() {
		if ctx.Err() != nil {
			return
		}
		choices := container.NewVBox()
		var picker *dialog.CustomDialog
		for _, name := range ctrl.Profiles().Names() {
			name := name
			choices.Add(widget.NewButton("Use profile "+name, func() {
				if ctx.Err() != nil {
					return
				}
				if !ctrl.Profiles().SetActive(name) {
					dialog.ShowError(fmt.Errorf("profile is no longer available: %s", name), parent)
					return
				}
				button.SetText("Profile: " + ctrl.Profiles().ActiveName())
				picker.Hide()
				if err := ctrl.Profiles().LastError(); err != nil {
					dialog.ShowError(err, parent)
				}
			}))
		}
		picker = dialog.NewCustomWithoutButtons("Choose Profile", choices, parent)
		picker.SetButtons([]fyne.CanvasObject{widget.NewButton("Close", func() { picker.Hide() })})
		picker.Show()
	}
	return button
}

func ShowConfigEditor(parentCtx context.Context, parent fyne.Window, ctrl *controller.Controller) {
	showConfigEditor(parentCtx, parent, ctrl, newQuitAction(parentCtx, ctrl.Cancel, ctrl.IsRunning, fyne.Do, fyne.CurrentApp().Quit))
}

func showConfigEditor(parentCtx context.Context, parent fyne.Window, ctrl *controller.Controller, quit func()) {
	p := ctrl.Profiles().Active()

	ctx, cancel := context.WithCancel(parentCtx)
	w := fyne.CurrentApp().NewWindow("Settings")
	cleanupAccessibility := func() {}
	w.SetOnClosed(func() { cleanupAccessibility(); cancel() })
	w.Resize(fyne.NewSize(900, 680))
	registerQuitShortcut(ctx, w, quit)
	saveStatus := widget.NewLabel("")
	saveStatus.Wrapping = fyne.TextWrapWord
	if err := ctrl.Profiles().LastError(); err != nil {
		saveStatus.SetText("Settings not saved: " + err.Error())
	}
	save := func() {
		if err := ctrl.Profiles().UpdateActive(p); err != nil {
			saveStatus.SetText("Settings not saved: " + err.Error())
		} else {
			saveStatus.SetText("Settings saved")
		}
	}

	// roots editor (checkbox table-like)
	rootsBox := container.NewVBox(widget.NewLabelWithStyle("Scan Roots", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	for i := range p.ScanRoots {
		idx := i
		check := widget.NewCheck(fmt.Sprintf("Enable root %d", i+1), func(v bool) {
			p.ScanRoots[idx].Enabled = v
			save()
		})
		check.SetChecked(p.ScanRoots[i].Enabled)

		pathEntry := widget.NewEntry()
		pathEntry.SetPlaceHolder("Folder path")
		pathEntry.SetText(p.ScanRoots[i].Path)
		pathEntry.OnChanged = func(s string) {
			p.ScanRoots[idx].Path = s
			save()
		}

		browse := widget.NewButton(fmt.Sprintf("Browse root %d", i+1), func() { showDirectoryPicker(ctx, w, pathEntry.Text, func(path string) { pathEntry.SetText(path) }) })

		rootsBox.Add(container.NewBorder(nil, nil, check, browse, pathEntry))
	}

	addRootBtn := widget.NewButton("+ Add Root", func() {
		p.ScanRoots = append(p.ScanRoots, storage.ScanRoot{Path: "", Enabled: true})
		save()
		w.Close()
		ShowConfigEditor(parentCtx, parent, ctrl)
	})

	excludes := widget.NewMultiLineEntry()
	excludes.SetPlaceHolder("Excluded directory names, one per line")
	excludes.SetText(strings.Join(p.ExcludeDirs, "\n"))
	excludes.OnChanged = func(s string) {
		p.ExcludeDirs = splitLines(s)
		save()
	}

	maxFile := widget.NewEntry()
	maxFile.SetPlaceHolder("Maximum file size in MB; 0 means unlimited")
	maxFile.SetText(fmt.Sprintf("%d", p.MaxFileSizeMB))
	maxFile.OnChanged = func(s string) {
		if v, err := parseInt64(s); err == nil {
			p.MaxFileSizeMB = v
			save()
		} else {
			saveStatus.SetText("Max file size must be a nonnegative integer")
		}
	}

	hashCheck := widget.NewCheck("Enable file hashing", func(v bool) {
		p.HashFiles = v
		save()
		// live mid-scan apply
		ctrl.Runtime().HashFiles.Store(v)
	})
	hashCheck.SetChecked(p.HashFiles)

	hashMax := widget.NewEntry()
	hashMax.SetPlaceHolder("Maximum hashing size in MB; 0 means unlimited")
	hashMax.SetText(fmt.Sprintf("%d", p.HashMaxFileSizeMB))
	hashMax.OnChanged = func(s string) {
		if v, err := parseInt64(s); err == nil {
			p.HashMaxFileSizeMB = v
			save()
			ctrl.Runtime().HashMaxSizeBytes.Store(v * 1024 * 1024)
		} else {
			saveStatus.SetText("Hash size limit must be a nonnegative integer")
		}
	}

	projCheck := widget.NewCheck("Detect projects", func(v bool) {
		p.ProjectDetection = v
		save()
		ctrl.Runtime().ProjectDetection.Store(v)
	})
	projCheck.SetChecked(p.ProjectDetection)

	cpCheck := widget.NewCheck("Save checkpoint", func(v bool) {
		p.CheckpointEnabled = v
		save()
	})
	cpCheck.SetChecked(p.CheckpointEnabled)

	cpPath := widget.NewEntry()
	cpPath.SetPlaceHolder("Checkpoint file path")
	cpPath.SetText(p.CheckpointPath)
	cpPath.OnChanged = func(s string) {
		p.CheckpointPath = s
		save()
	}

	validateBtn := widget.NewButton("Validate", func() {
		errs := controller.ValidateProfile(p)
		if len(errs) > 0 {
			dialog.ShowError(fmt.Errorf("%s", controller.JoinErrors(errs)), w)
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
		widget.NewFormItem("Project Detection (live; applied at completion)", projCheck),
		widget.NewFormItem("Checkpoint Enabled", cpCheck),
		widget.NewFormItem("Checkpoint Path", cpPath),
	)

	behaviorHint := widget.NewLabel("Roots, exclusions and file-size changes apply to the next scan. Live hashing changes apply to subsequent files.")
	behaviorHint.Wrapping = fyne.TextWrapWord
	content := container.NewVScroll(container.NewVBox(
		behaviorHint,
		rootsBox,
		widget.NewSeparator(),
		form,
		validateBtn,
		saveStatus,
	))
	w.SetContent(content)
	w.Show()
	cleanupAccessibility = attachWindowAccessibility(ctx, w)
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
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, err
	}
	if v < 0 {
		return 0, fmt.Errorf("value must be nonnegative")
	}
	return v, nil
}
