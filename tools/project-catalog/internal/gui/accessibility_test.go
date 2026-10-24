//go:build cgo

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"scan-projects/internal/nativea11y"
	"testing"
)

func semanticNode(t *testing.T, s nativea11y.Snapshot, name string) nativea11y.Node {
	t.Helper()
	for _, n := range s.Nodes {
		if n.Name == name {
			return n
		}
	}
	t.Fatalf("missing semantic control %q in %+v", name, s.Nodes)
	return nativea11y.Node{}
}
func TestSemanticActionsReuseCallbacksAndCurrentEnabledState(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	called := 0
	b := widget.NewButton("Start", func() { called++ })
	entry := widget.NewEntry()
	entry.PlaceHolder = "Folder path"
	edited := ""
	entry.OnChanged = func(v string) { edited = v }
	check := widget.NewCheck("Hash files", nil)
	w := test.NewWindow(container.NewVBox(b, entry, check))
	defer w.Close()
	w.Resize(fyne.NewSize(500, 300))
	m := newSemanticWindow(w)
	snap := m.snapshot()
	start := semanticNode(t, snap, "Start")
	m.perform(nativea11y.Action{NodeID: start.ID, Kind: "click"})
	if called != 1 {
		t.Fatal("native click did not reuse callback")
	}
	b.Disable()
	m.perform(nativea11y.Action{NodeID: start.ID, Kind: "click"})
	if called != 1 {
		t.Fatal("stale native action bypassed current disabled state")
	}
	field := semanticNode(t, snap, "Folder path")
	m.perform(nativea11y.Action{NodeID: field.ID, Kind: "set_value", Value: "/synthetic/root"})
	if edited != "/synthetic/root" || entry.Text != edited {
		t.Fatal("native value did not reuse entry state/callback")
	}
	n := semanticNode(t, snap, "Hash files")
	m.perform(nativea11y.Action{NodeID: n.ID, Kind: "click"})
	if !check.Checked {
		t.Fatal("native checkbox action failed")
	}
	m.perform(nativea11y.Action{NodeID: field.ID, Kind: "focus"})
	if w.Canvas().Focused() != entry {
		t.Fatal("native focus did not move actual keyboard focus")
	}
	if semanticNode(t, m.snapshot(), "Start").ID != start.ID {
		t.Fatal("control ID changed on refresh")
	}
}
func TestSemanticStatusAndModalExposeActualCurrentContent(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	label := widget.NewLabel("running | files=2 | hashed=2")
	w := test.NewWindow(label)
	defer w.Close()
	m := newSemanticWindow(w)
	n := semanticNode(t, m.snapshot(), "Scan status")
	if n.Role != "status" || n.Live != "polite" || n.Value != label.Text {
		t.Fatal(n)
	}
	popup := widget.NewModalPopUp(container.NewVBox(widget.NewLabel("Choose folder"), widget.NewButton("Select folder", nil)), w.Canvas())
	popup.Show()
	snap := m.snapshot()
	semanticNode(t, snap, "Select folder")
	for _, n := range snap.Nodes {
		if n.Name == "Scan status" {
			t.Fatal("modal exposed background controls")
		}
	}
	popup.Hide()
}
func TestSemanticTreeSelectionExpansionAndStableIDs(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	selected := ""
	tree := widget.NewTree(func(uid string) []string {
		if uid == "" {
			return []string{"branch"}
		}
		if uid == "branch" {
			return []string{"/synthetic/a [true]"}
		}
		return nil
	}, func(uid string) bool { return uid == "branch" }, func(bool) fyne.CanvasObject { return widget.NewLabel("") }, func(uid string, _ bool, o fyne.CanvasObject) { o.(*widget.Label).SetText(uid) })
	tree.Root = ""
	tree.OnSelected = func(uid string) { selected = uid }
	w := test.NewWindow(tree)
	defer w.Close()
	w.Resize(fyne.NewSize(500, 300))
	m := newSemanticWindow(w)
	root := semanticNode(t, m.snapshot(), "branch")
	m.perform(nativea11y.Action{NodeID: root.ID, Kind: "expand"})
	snap := m.snapshot()
	child := semanticNode(t, snap, "/synthetic/a [true]")
	m.perform(nativea11y.Action{NodeID: child.ID, Kind: "select"})
	if selected != "/synthetic/a [true]" || w.Canvas().Focused() != tree {
		t.Fatal("tree native selection did not use actual tree")
	}
	snap = m.snapshot()
	if snap.Focus != child.ID || !semanticNode(t, snap, child.Name).Selected {
		t.Fatal("tree selected/focused state missing")
	}
	m.perform(nativea11y.Action{NodeID: root.ID, Kind: "collapse"})
	for _, n := range m.snapshot().Nodes {
		if n.ID == child.ID {
			t.Fatal("collapsed child still exposed")
		}
	}
	m.perform(nativea11y.Action{NodeID: root.ID, Kind: "expand"})
	if semanticNode(t, m.snapshot(), child.Name).ID != child.ID {
		t.Fatal("tree identity changed")
	}
}

func TestStaleNativeActionCannotInvokeBackgroundOrRemovedControl(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	calls := 0
	button := widget.NewButton("Start", func() { calls++ })
	content := container.NewVBox(button)
	w := test.NewWindow(content)
	defer w.Close()
	m := newSemanticWindow(w)
	id := semanticNode(t, m.snapshot(), "Start").ID
	popup := widget.NewModalPopUp(widget.NewLabel("Modal owns actions"), w.Canvas())
	popup.Show()
	m.perform(nativea11y.Action{NodeID: id, Kind: "click"})
	if calls != 0 {
		t.Fatal("stale native action invoked a background control")
	}
	popup.Hide()
	content.Remove(button)
	m.perform(nativea11y.Action{NodeID: id, Kind: "click"})
	if calls != 0 {
		t.Fatal("stale native action invoked removed control")
	}
}

func TestNavigationAndPersistenceErrorsAreLiveFeedback(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	label := widget.NewLabel("Choose a folder")
	w := test.NewWindow(label)
	defer w.Close()
	model := newSemanticWindow(w)
	for _, text := range []string{"Choose a folder", "Loading folders…", "Cannot open folder: synthetic denial", "Current folder: /synthetic · 0 subfolders", "Settings not saved: synthetic denial", "Settings saved", "Max file size must be a nonnegative integer"} {
		label.SetText(text)
		node := semanticNode(t, model.snapshot(), text)
		if node.Live != "polite" {
			t.Fatalf("message not announced: %q", text)
		}
	}
	label.SetText("ordinary project name")
	if semanticNode(t, model.snapshot(), label.Text).Live != "" {
		t.Fatal("ordinary label announced")
	}
}

func TestNativeTextRangesRespectCurrentModalAndDisabledState(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	entry := widget.NewEntry()
	entry.PlaceHolder = "Synthetic path"
	entry.SetText("aé🙂z")
	w := test.NewWindow(entry)
	defer w.Close()
	model := newSemanticWindow(w)
	node := semanticNode(t, model.snapshot(), entry.PlaceHolder)
	model.perform(nativea11y.Action{NodeID: node.ID, Kind: "set_text_selection", Selection: &nativea11y.TextSelection{Anchor: 1, Focus: 3}})
	if entry.SelectedText() != "é🙂" {
		t.Fatal("native range not applied")
	}
	modal := widget.NewModalPopUp(widget.NewLabel("Modal"), w.Canvas())
	modal.Show()
	model.perform(nativea11y.Action{NodeID: node.ID, Kind: "set_text_selection", Selection: &nativea11y.TextSelection{Anchor: 0, Focus: 0}})
	if entry.SelectedText() != "é🙂" {
		t.Fatal("stale modal action applied")
	}
	modal.Hide()
	entry.Disable()
	model.perform(nativea11y.Action{NodeID: node.ID, Kind: "set_text_selection", Selection: &nativea11y.TextSelection{Anchor: 0, Focus: 0}})
	if entry.SelectedText() != "é🙂" {
		t.Fatal("disabled action applied")
	}
}

func TestInvalidNativeRangeDoesNotStealFocus(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	entry := widget.NewEntry()
	entry.PlaceHolder = "Synthetic path"
	entry.SetText("aé🙂z")
	button := widget.NewButton("Other", nil)
	w := test.NewWindow(container.NewVBox(entry, button))
	defer w.Close()
	model := newSemanticWindow(w)
	node := semanticNode(t, model.snapshot(), entry.PlaceHolder)
	w.Canvas().Focus(button)
	model.perform(nativea11y.Action{NodeID: node.ID, Kind: "set_text_selection", Selection: &nativea11y.TextSelection{Anchor: 0, Focus: 5}})
	if w.Canvas().Focused() != button {
		t.Fatal("invalid range stole keyboard focus")
	}
}
