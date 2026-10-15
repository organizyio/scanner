//go:build cgo

package gui

import (
	"context"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	fynetest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"os"
	"path/filepath"
	"scan-projects/internal/controller"
	"scan-projects/internal/nativea11y"
	"scan-projects/internal/storage"
	"strings"
	"testing"
	"time"
)

func TestFolderTreeKeyboardSelectsStableUnmodifiedPath(t *testing.T) {
	home := t.TempDir()
	for _, name := range []string{"HOME", "XDG_CONFIG_HOME", "APPDATA", "USERPROFILE"} {
		t.Setenv(name, home)
	}
	app := fynetest.NewApp()
	defer app.Quit()
	ctrl := controller.New(app)
	root := filepath.Join(t.TempDir(), "folder [true]")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	profile := ctrl.Profiles().Active()
	profile.ScanRoots = []storage.ScanRoot{{Path: root, Enabled: true}}
	if err := ctrl.Profiles().UpdateActive(profile); err != nil {
		t.Fatal(err)
	}
	ids := treeChildren(ctrl, "")
	if len(ids) != 1 || ids[0] != root {
		t.Fatalf("unstable folder IDs %v", ids)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	details := widget.NewLabel("")
	tree := NewFolderTree(ctx, ctrl, details)
	selected := ""
	tree.OnSelected = func(id string) { selected = id; showFolderDetails(ctrl, id, details) }
	tree.FocusGained()
	tree.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	if selected != root {
		t.Fatalf("keyboard selection %q want %q", selected, root)
	}
	if details.Text != root+"\n\n(no data yet)" {
		t.Fatalf("path label was stripped: %q", details.Text)
	}
}

func TestSettingsNumbersRejectMalformedValues(t *testing.T) {
	for _, input := range []string{"-1", "1oops", "", "1.5"} {
		if _, err := parseInt64(input); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
	if value, err := parseInt64(" 0 "); err != nil || value != 0 {
		t.Fatalf("zero: %d %v", value, err)
	}
}

func TestFolderPickerDirectoryListAndErrors(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "Unicode-Ա [true]")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	paths, err := directoryPaths(root)
	if err != nil || len(paths) != 1 || paths[0] != child {
		t.Fatalf("directories %v %v", paths, err)
	}
	if _, err := directoryPaths(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing directory error hidden")
	}
}

func TestFolderTreeArrowNavigationSelectsTheVisuallyFocusedRow(t *testing.T) {
	app := fynetest.NewApp()
	defer app.Quit()
	children := map[string][]string{"": {"branch", "sibling"}, "branch": {"branch/child"}}
	base := widget.NewTree(func(uid string) []string { return children[uid] }, func(uid string) bool { return len(children[uid]) > 0 }, func(bool) fyne.CanvasObject { return widget.NewLabel("") }, func(uid string, _ bool, o fyne.CanvasObject) { o.(*widget.Label).SetText(uid) })
	selected := ""
	base.OnSelected = func(uid string) { selected = uid }
	tree := newFolderTree(base)
	window := fynetest.NewWindow(tree)
	defer window.Close()
	window.Canvas().Focus(tree)
	key := func(name fyne.KeyName, want string) {
		t.Helper()
		tree.TypedKey(&fyne.KeyEvent{Name: name})
		if selected != want || tree.selected != want {
			t.Fatalf("%s selected=%s visual=%s want=%s", name, selected, tree.selected, want)
		}
	}
	if selected != "branch" {
		t.Fatalf("initial focus %q", selected)
	}
	key(fyne.KeyRight, "branch")
	if !tree.IsBranchOpen("branch") {
		t.Fatal("Right did not expand")
	}
	key(fyne.KeyDown, "branch/child")
	base.CloseBranch("branch")
	if selected != "branch" {
		t.Fatal("collapsing selected child did not focus its parent")
	}
	base.OpenBranch("branch")
	key(fyne.KeyDown, "branch/child")
	key(fyne.KeyLeft, "branch")
	key(fyne.KeyLeft, "branch")
	if tree.IsBranchOpen("branch") {
		t.Fatal("Left did not collapse")
	}
	key(fyne.KeyDown, "sibling")
	key(fyne.KeyUp, "branch")
	key(fyne.KeyReturn, "branch")
	key(fyne.KeySpace, "branch")
}

func TestProfilePickerUsesActualNamedActionsAndPersistsChoice(t *testing.T) {
	home := t.TempDir()
	for _, name := range []string{"HOME", "XDG_CONFIG_HOME", "APPDATA", "USERPROFILE"} {
		t.Setenv(name, home)
	}
	app := fynetest.NewApp()
	defer app.Quit()
	ctrl := controller.New(app)
	original := ctrl.Profiles().ActiveName()
	if !ctrl.Profiles().New("Secondary") {
		t.Fatal("fixture profile")
	}
	ctrl.Profiles().SetActive(original)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	window := fynetest.NewWindow(nil)
	defer window.Close()
	picker := NewProfilesSelect(ctx, window, ctrl)
	window.SetContent(picker)
	picker.OnTapped()
	model := newSemanticWindow(window)
	var actionID uint64
	for _, node := range model.snapshot().Nodes {
		if node.Name == "Use profile Secondary" {
			actionID = node.ID
		}
	}
	if actionID == 0 {
		t.Fatal("profile choice missing from actual modal")
	}
	model.perform(nativea11y.Action{NodeID: actionID, Kind: "click"})
	if ctrl.Profiles().ActiveName() != "Secondary" || picker.Text != "Profile: Secondary" {
		t.Fatal("profile action did not update actual controls")
	}
	config, err := storage.LoadYAML[storage.AppConfig](storage.SettingsPath())
	if err != nil || config.ActiveProfile != "Secondary" {
		t.Fatalf("profile not persisted: %v", err)
	}
}

func TestFolderPickerIgnoresQueuedResultAfterTypingAndRecoversUp(t *testing.T) {
	app := fynetest.NewApp()
	defer app.Quit()
	window := fynetest.NewWindow(widget.NewLabel("Synthetic parent"))
	defer window.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	root := t.TempDir()
	missing := filepath.Join(root, "missing")
	queued := make(chan func(), 4)
	controls := newDirectoryPicker(ctx, window, root, func(string) {}, func(path string) ([]string, error) {
		if path == missing {
			return nil, os.ErrNotExist
		}
		return nil, nil
	}, func(f func()) { queued <- f })
	defer controls.picker.Hide()
	next := func() func() {
		t.Helper()
		select {
		case apply := <-queued:
			return apply
		case <-time.After(time.Second):
			t.Fatal("directory result not queued")
			return nil
		}
	}
	stale := next()
	controls.path.SetText(missing)
	stale()
	if !controls.selectFolder.Disabled() || !strings.Contains(controls.status.Text, "Press Enter") {
		t.Fatal("stale read enabled selection under edited path")
	}
	controls.path.SetText(root)
	stale()
	if !controls.selectFolder.Disabled() {
		t.Fatal("returning to old text revived old result")
	}
	controls.goButton.OnTapped()
	next()()
	if controls.selectFolder.Disabled() {
		t.Fatal("new valid directory request not applied")
	}
	controls.path.SetText(missing)
	controls.goButton.OnTapped()
	next()()
	if !controls.selectFolder.Disabled() || controls.up.Disabled() {
		t.Fatal("invalid folder should disable selection and retain Up recovery")
	}
	controls.up.OnTapped()
	next()()
	if controls.selectFolder.Disabled() || controls.path.Text != filepath.Dir(root) {
		t.Fatal("Up did not recover the previous valid parent")
	}
}

func TestFolderTreeHasDistinctVisibleFocusWithoutChangingSelection(t *testing.T) {
	app := fynetest.NewApp()
	defer app.Quit()
	tree := newFolderTree(widget.NewTree(func(uid string) []string {
		if uid == "" {
			return []string{"selected row"}
		}
		return nil
	}, func(string) bool { return false }, func(bool) fyne.CanvasObject { return widget.NewLabel("") }, func(uid string, _ bool, o fyne.CanvasObject) { o.(*widget.Label).SetText(uid) }))
	other := widget.NewButton("Other control", nil)
	window := fynetest.NewWindow(container.NewVBox(tree, other))
	defer window.Close()
	tree.Resize(fyne.NewSize(400, 180))
	renderer, ok := fynetest.WidgetRenderer(tree).(*folderTreeRenderer)
	if !ok {
		t.Fatal("focus wrapper renderer missing")
	}
	window.Canvas().Focus(tree)
	if !renderer.border.Visible() {
		t.Fatal("focused tree has no visible boundary")
	}
	selected := tree.selected
	_, _, _, alpha := renderer.border.StrokeColor.RGBA()
	if alpha != 65535 {
		t.Fatal("focus accent is translucent")
	}
	if colorContrast(renderer.border.StrokeColor, theme.Color(theme.ColorNameBackground)) < 3 {
		t.Fatal("focus boundary lacks contrast against tree background")
	}
	renderer.Layout(fyne.NewSize(400, 180))
	if renderer.border.Position().X <= 0 || renderer.border.Size().Width >= 400 {
		t.Fatal("focus stroke clips outside widget")
	}
	window.Canvas().Focus(other)
	if renderer.border.Visible() {
		t.Fatal("unfocused tree still has focus boundary")
	}
	if tree.selected != selected || selected != "selected row" {
		t.Fatal("focus changed retained selection")
	}
	window.Canvas().Focus(tree)
	if !renderer.border.Visible() || tree.selected != selected {
		t.Fatal("returning focus did not restore boundary independently of selection")
	}
}
