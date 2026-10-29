//go:build cgo

package gui

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	fynetest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"scan-projects/internal/controller"
	"scan-projects/internal/nativea11y"
	"scan-projects/internal/storage"
)

func TestLongRootLabelsKeepLeafNamesAndCompleteNativePaths(t *testing.T) {
	home := t.TempDir()
	for _, name := range []string{"HOME", "XDG_CONFIG_HOME", "APPDATA", "USERPROFILE"} {
		t.Setenv(name, home)
	}
	app := fynetest.NewApp()
	defer app.Quit()
	ctrl := controller.New(app)
	parent := filepath.Join(home, strings.Repeat("shared-parent-", 12))
	first, second := filepath.Join(parent, "Unicode-Δ-日本語"), filepath.Join(parent, "different-leaf")
	profile := ctrl.Profiles().Active()
	profile.ScanRoots = []storage.ScanRoot{{Path: first, Enabled: true}, {Path: second, Enabled: false}}
	if err := ctrl.Profiles().UpdateActive(profile); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	details := widget.NewLabel("")
	tree := NewFolderTree(ctx, ctrl, details)
	window := fynetest.NewWindow(tree)
	defer window.Close()
	window.Resize(fyne.NewSize(360, 260))
	model := newSemanticWindow(window)
	for i, path := range []string{first, second} {
		visual := tree.CreateNode(false).(*widget.Label)
		tree.UpdateNode(path, false, visual)
		if !strings.HasPrefix(visual.Text, filepath.Base(path)+" (") || visual.Truncation != fyne.TextTruncateEllipsis || visual.Wrapping != fyne.TextWrapOff {
			t.Fatalf("unreadable root %q", visual.Text)
		}
		state := " (enabled)"
		if i == 1 {
			state = " (disabled)"
		}
		node := semanticNode(t, model.snapshot(), path+state)
		model.perform(nativea11y.Action{NodeID: node.ID, Kind: "select"})
		if details.Text != path+"\n\n(no data yet)" {
			t.Fatalf("full selected path lost: %q", details.Text)
		}
	}
}

func TestSettingsBrowseHasOuterPadding(t *testing.T) {
	home := t.TempDir()
	for _, name := range []string{"HOME", "XDG_CONFIG_HOME", "APPDATA", "USERPROFILE"} {
		t.Setenv(name, home)
	}
	app := fynetest.NewApp()
	defer app.Quit()
	ctrl := controller.New(app)
	profile := ctrl.Profiles().Active()
	profile.ScanRoots = []storage.ScanRoot{{Path: filepath.Join(home, "long-root"), Enabled: true}}
	if err := ctrl.Profiles().UpdateActive(profile); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	parent := fynetest.NewWindow(widget.NewLabel("Synthetic parent"))
	defer parent.Close()
	showConfigEditor(ctx, parent, ctrl, func() {})
	var settings fyne.Window
	for _, w := range app.Driver().AllWindows() {
		if w.Title() == "Settings" {
			settings = w
		}
	}
	if settings == nil {
		t.Fatal("Settings window missing")
	}
	defer settings.Close()
	settings.Resize(fyne.NewSize(900, 680))
	node := semanticNode(t, newSemanticWindow(settings).snapshot(), "Browse root 1")
	right := float64(settings.Canvas().Size().Width) - node.Bounds.X - node.Bounds.Width
	if node.Bounds.Width <= 0 || right < float64(theme.Padding()) {
		t.Fatalf("Browse root touches Settings edge: right=%v bounds=%+v", right, node.Bounds)
	}
}
