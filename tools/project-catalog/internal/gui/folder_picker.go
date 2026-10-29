//go:build cgo

package gui

import (
	"context"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"os"
	"path/filepath"
	"scan-projects/internal/controller"
	"scan-projects/internal/storage"
)

// ShowFolderPicker adds a root without replacing roots or changing settings identity.
func ShowFolderPicker(ctx context.Context, parent fyne.Window, ctrl *controller.Controller, onAdded func(string)) {
	initial, _ := os.UserHomeDir()
	for _, root := range ctrl.Profiles().Active().ScanRoots {
		if info, err := os.Stat(root.Path); err == nil && info.IsDir() {
			initial = root.Path
			break
		}
	}
	showDirectoryPicker(ctx, parent, initial, func(path string) {
		profile := ctrl.Profiles().Active()
		found := false
		for i, root := range profile.ScanRoots {
			if filepath.Clean(root.Path) == path {
				profile.ScanRoots[i].Enabled = true
				found = true
				break
			}
		}
		if !found {
			profile.ScanRoots = append(profile.ScanRoots, storage.ScanRoot{Path: path, Enabled: true})
		}
		if err := ctrl.Profiles().UpdateActive(profile); err != nil {
			dialog.ShowError(err, parent)
			return
		}
		if onAdded != nil {
			onAdded(path)
		}
	})
}

// App-owned controls expose the same folder selection to keyboard and native AT.
// Directory loading runs off the UI thread; stale/closed requests cannot update widgets.
type folderPickerControls struct {
	path                       *widget.Entry
	selectFolder, up, goButton *widget.Button
	status                     *widget.Label
	picker                     *dialog.CustomDialog
}

func showDirectoryPicker(parentCtx context.Context, parent fyne.Window, initial string, onChosen func(string)) {
	newDirectoryPicker(parentCtx, parent, initial, onChosen, directoryPaths, fyne.Do)
}

func newDirectoryPicker(parentCtx context.Context, parent fyne.Window, initial string, onChosen func(string), readDirectory func(string) ([]string, error), dispatch func(func())) *folderPickerControls {
	ctx, cancel := context.WithCancel(parentCtx)
	path := widget.NewEntry()
	path.SetPlaceHolder("Folder path: type a path, then Enter or Go")
	status := widget.NewLabel("Choose a folder")
	status.Wrapping = fyne.TextWrapOff
	directories := container.NewVBox()
	scroll := container.NewVScroll(directories)
	scroll.SetMinSize(fyne.NewSize(620, 300))
	current := ""
	requests := directoryRequests{}
	var picker *dialog.CustomDialog
	selectFolder := widget.NewButton("Select Folder", func() {
		if ctx.Err() != nil {
			return
		}
		if current != "" {
			picker.Hide()
			onChosen(current)
		}
	})
	selectFolder.Disable()
	var navigate func(string)
	up := widget.NewButton("Up", func() {
		if current != "" {
			navigate(filepath.Dir(current))
		}
	})
	up.Disable()
	goButton := widget.NewButton("Go", func() { navigate(path.Text) })
	navigate = func(target string) {
		if ctx.Err() != nil {
			return
		}
		absolute, err := filepath.Abs(target)
		if err != nil {
			status.SetText("Cannot open folder: " + err.Error())
			selectFolder.Disable()
			return
		}
		absolute = filepath.Clean(absolute)
		path.SetText(absolute)
		request := requests.begin()
		up.Disable()
		directories.RemoveAll()
		selectFolder.Disable()
		status.SetText("Loading folders…")
		go func() {
			names, err := readDirectory(absolute)
			if ctx.Err() != nil {
				return
			}
			dispatch(func() {
				if ctx.Err() != nil {
					return
				}
				requests.apply(request, func() {
					if err != nil {
						status.SetText("Cannot open folder: " + err.Error())
						if current != "" && filepath.Dir(current) != current {
							up.Enable()
						}
						return
					}
					current = absolute
					directories.RemoveAll()
					for _, child := range names {
						child := child
						directories.Add(widget.NewButton("Open "+filepath.Base(child), func() { navigate(child) }))
					}
					selectFolder.Enable()
					if filepath.Dir(current) == current {
						up.Disable()
					} else {
						up.Enable()
					}
					status.SetText(fmt.Sprintf("Current folder: %s · %d subfolders", current, len(names)))
				})
			})
		}()
	}
	path.OnChanged = func(string) {
		requests.invalidate()
		selectFolder.Disable()
		status.SetText("Press Enter or Go to open the typed folder")
	}
	path.OnSubmitted = navigate
	cancelButton := widget.NewButton("Cancel", func() { picker.Hide() })
	navigation := container.NewBorder(nil, nil, up, goButton, path)
	content := container.NewBorder(container.NewVBox(widget.NewLabel("Folder path"), navigation, container.NewHScroll(status)), container.NewHBox(selectFolder, cancelButton), nil, nil, scroll)
	picker = dialog.NewCustomWithoutButtons("Choose Folder", content, parent)
	picker.SetOnClosed(cancel)
	picker.Resize(fyne.NewSize(720, 480))
	picker.Show()
	parent.Canvas().Focus(path)
	if initial == "" {
		initial = "."
	}
	navigate(initial)
	return &folderPickerControls{path: path, selectFolder: selectFolder, up: up, goButton: goButton, status: status, picker: picker}
}

func directoryPaths(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		child := filepath.Join(path, entry.Name())
		isDir := entry.IsDir()
		if !isDir && entry.Type()&os.ModeSymlink != 0 {
			if info, err := os.Stat(child); err == nil {
				isDir = info.IsDir()
			}
		}
		if isDir {
			paths = append(paths, child)
		}
	}
	return paths, nil
}
