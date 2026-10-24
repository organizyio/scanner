//go:build cgo

package gui

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"scan-projects/internal/nativea11y"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// semanticWindow mirrors actual controls. Native actions reuse their existing UI
// callbacks; snapshots and handlers always execute on the Fyne event loop.
type semanticWindow struct {
	window  fyne.Window
	next    uint64
	ids     map[fyne.CanvasObject]uint64
	keys    map[string]uint64
	actions map[uint64]func(nativea11y.Action)
	trees   map[*widget.Tree]*semanticTree
	nodes   []nativea11y.Node
	focus   uint64
}
type semanticTree struct {
	labels   map[string]fyne.CanvasObject
	owners   map[fyne.CanvasObject]string
	selected string
	update   func(string, bool, fyne.CanvasObject)
}

func newSemanticWindow(w fyne.Window) *semanticWindow {
	return &semanticWindow{window: w, next: 2, ids: make(map[fyne.CanvasObject]uint64), keys: make(map[string]uint64), actions: make(map[uint64]func(nativea11y.Action)), trees: make(map[*widget.Tree]*semanticTree)}
}
func (s *semanticWindow) id(o fyne.CanvasObject) uint64 {
	if n := s.ids[o]; n != 0 {
		return n
	}
	n := s.next
	s.next++
	s.ids[o] = n
	return n
}
func (s *semanticWindow) key(k string) uint64 {
	if n := s.keys[k]; n != 0 {
		return n
	}
	n := s.next
	s.next++
	s.keys[k] = n
	return n
}
func (s *semanticWindow) bounds(o fyne.CanvasObject) nativea11y.Bounds {
	if fyne.CurrentApp().Driver().CanvasForObject(o) == nil {
		return nativea11y.Bounds{}
	}
	p := fyne.CurrentApp().Driver().AbsolutePositionForObject(o)
	z := o.Size()
	return nativea11y.Bounds{X: float64(p.X), Y: float64(p.Y), Width: float64(z.Width), Height: float64(z.Height)}
}
func (s *semanticWindow) add(o fyne.CanvasObject, parent uint64, role, name, value string, acts []string, action func(nativea11y.Action)) nativea11y.Node {
	n := nativea11y.Node{ID: s.id(o), Parent: parent, Role: role, Name: name, Value: value, Bounds: s.bounds(o), Actions: acts}
	if d, ok := o.(fyne.Disableable); ok {
		n.Disabled = d.Disabled()
	}
	if f, ok := o.(fyne.Focusable); ok && s.window.Canvas().Focused() == f {
		s.focus = n.ID
	}
	s.actions[n.ID] = func(a nativea11y.Action) {
		if d, ok := o.(fyne.Disableable); ok && d.Disabled() {
			return
		}
		if a.Kind == "focus" {
			if f, ok := o.(fyne.Focusable); ok {
				s.window.Canvas().Focus(f)
			}
			return
		}
		if action != nil {
			action(a)
		}
	}
	return n
}
func (s *semanticWindow) visit(o fyne.CanvasObject, parent uint64, label string) {
	if o == nil || !o.Visible() {
		return
	}
	name := label
	switch v := o.(type) {
	case *fyne.Container:
		n := s.add(o, parent, "group", name, "", nil, nil)
		s.nodes = append(s.nodes, n)
		for _, c := range v.Objects {
			s.visit(c, n.ID, "")
		}
	case *container.Scroll:
		s.visit(v.Content, parent, label)
	case *container.Split:
		s.visit(v.Leading, parent, "")
		s.visit(v.Trailing, parent, "")
	case *widget.PopUp:
		s.visit(v.Content, parent, "Dialog")
	case *widget.Form:
		n := s.add(o, parent, "group", name, "", nil, nil)
		s.nodes = append(s.nodes, n)
		for _, f := range v.Items {
			s.visit(f.Widget, n.ID, f.Text)
		}
	case *widget.Button:
		if name == "" {
			name = v.Text
		}
		n := s.add(o, parent, "button", name, "", []string{"click", "focus"}, func(a nativea11y.Action) {
			if a.Kind == "click" && v.OnTapped != nil {
				v.OnTapped()
			}
		})
		s.nodes = append(s.nodes, n)
	case *widget.Entry:
		if name == "" {
			name = v.PlaceHolder
		}
		if name == "" {
			name = "Text"
		}
		actions := []string{"set_value", "focus"}
		if entrySupportsSelection(v) {
			actions = append(actions, "set_text_selection")
		}
		n := s.add(o, parent, "entry", name, v.Text, actions, func(a nativea11y.Action) {
			if a.Kind == "set_value" {
				v.SetText(a.Value)
			}
			if a.Kind == "set_text_selection" {
				s.window.Canvas().Focus(v)
				setEntrySelection(v, a.Selection)
			}
		})
		n.Selection = entrySelection(v)
		n.Placeholder = v.PlaceHolder
		s.nodes = append(s.nodes, n)
	case *widget.Check:
		if name == "" {
			name = v.Text
		}
		n := s.add(o, parent, "checkbox", name, "", []string{"click", "focus"}, func(a nativea11y.Action) {
			if a.Kind == "click" {
				v.SetChecked(!v.Checked)
			}
		})
		n.Checked = v.Checked
		s.nodes = append(s.nodes, n)
	case *widget.Select:
		if name == "" {
			name = v.PlaceHolder
		}
		if name == "" {
			name = "Profile"
		}
		n := s.add(o, parent, "combobox", name, v.Selected, []string{"click", "expand", "set_value", "focus"}, func(a nativea11y.Action) {
			switch a.Kind {
			case "click", "expand":
				v.Tapped(&fyne.PointEvent{})
			case "set_value":
				for _, option := range v.Options {
					if option == a.Value {
						v.SetSelected(option)
						return
					}
				}
			}
		})
		s.nodes = append(s.nodes, n)
	case *FolderTree:
		s.visitTree(v.Tree, parent, v)
	case *widget.Tree:
		s.visitTree(v, parent, v)
	case *widget.Label:
		role := "text"
		value := ""
		if strings.Contains(v.Text, " | files=") {
			role = "status"
			value = v.Text
			name = "Scan status"
		} else if name == "" {
			name = v.Text
		}
		if name != "" {
			n := s.add(o, parent, role, name, value, nil, nil)
			if role == "status" || liveFeedback(v.Text) {
				n.Live = "polite"
			}
			s.nodes = append(s.nodes, n)
		}
	case *widget.RichText:
		if name == "" {
			name = v.String()
		}
		if name != "" {
			s.nodes = append(s.nodes, s.add(o, parent, "text", name, "", nil, nil))
		}
	case *widget.PopUpMenu:
		n := s.add(o, parent, "group", "Menu", "", nil, nil)
		s.nodes = append(s.nodes, n)
		for _, object := range v.Menu.Items {
			// Menu.Items contains internal widgets with the exported menu Item.
			// Read only that field; never create another widget renderer.
			value := reflect.ValueOf(object)
			if value.Kind() != reflect.Pointer || value.IsNil() {
				continue
			}
			field := value.Elem().FieldByName("Item")
			if !field.IsValid() || !field.CanInterface() {
				continue
			}
			item, ok := field.Interface().(*fyne.MenuItem)
			if !ok || item == nil || item.IsSeparator {
				continue
			}
			node := s.add(object, n.ID, "button", item.Label, "", []string{"click"}, nil)
			node.Disabled = item.Disabled
			s.nodes = append(s.nodes, node)
			it := item
			s.actions[node.ID] = func(a nativea11y.Action) {
				if a.Kind == "click" && !it.Disabled && it.Action != nil {
					it.Action()
					v.Hide()
				}
			}
		}
	default:
		// Fyne overlays wrap their actual content in a widget with an exported
		// Content field. Unwrap that model field without touching renderers.
		value := reflect.ValueOf(o)
		if value.Kind() == reflect.Pointer && !value.IsNil() && value.Elem().Kind() == reflect.Struct {
			field := value.Elem().FieldByName("Content")
			if field.IsValid() && field.CanInterface() {
				if content, ok := field.Interface().(fyne.CanvasObject); ok && content != o {
					s.visit(content, parent, label)
				}
			}
		}
	}
}

// These are app-owned navigation/persistence messages, not arbitrary file names.
func liveFeedback(text string) bool {
	return strings.HasPrefix(text, "Cannot open folder:") || strings.HasPrefix(text, "Current folder:") ||
		strings.HasPrefix(text, "Settings not saved:") || text == "Settings saved" ||
		text == "Choose a folder" || text == "Loading folders…" || text == "Press Enter or Go to open the typed folder" ||
		text == "Max file size must be a nonnegative integer" || text == "Hash size limit must be a nonnegative integer"
}
func (s *semanticWindow) visitTree(t *widget.Tree, parent uint64, focusObject fyne.Focusable) {
	object := focusObject.(fyne.CanvasObject)
	n := s.add(object, parent, "tree", "Folders", "", []string{"focus"}, nil)
	s.nodes = append(s.nodes, n)
	binding := s.trees[t]
	if binding == nil {
		binding = &semanticTree{labels: map[string]fyne.CanvasObject{}, owners: map[fyne.CanvasObject]string{}, update: t.UpdateNode}
		s.trees[t] = binding
		original := t.OnSelected
		t.OnSelected = func(uid string) {
			binding.selected = uid
			if original != nil {
				original(uid)
			}
		}
		t.UpdateNode = func(uid string, branch bool, o fyne.CanvasObject) {
			if previous := binding.owners[o]; previous != "" && previous != uid {
				delete(binding.labels, previous)
			}
			binding.owners[o] = uid
			binding.labels[uid] = o
			if binding.update != nil {
				binding.update(uid, branch, o)
			}
		}
		t.Refresh()
	}
	seen := make(map[string]bool)
	var walk func(string, uint64)
	walk = func(uid string, p uint64) {
		if seen[uid] {
			return
		}
		seen[uid] = true
		id := s.key(fmt.Sprintf("tree:%d:%s", n.ID, uid))
		branch := t.IsBranch != nil && t.IsBranch(uid)
		label := uid
		bounds := nativea11y.Bounds{}
		if obj := binding.labels[uid]; obj != nil {
			if l, ok := obj.(*widget.Label); ok {
				label = l.Text
			}
			bounds = s.bounds(obj)
		} else if t.CreateNode != nil && binding.update != nil {
			template := t.CreateNode(branch)
			binding.update(uid, branch, template)
			if l, ok := template.(*widget.Label); ok {
				label = l.Text
			}
		}
		node := nativea11y.Node{ID: id, Parent: p, Role: "treeitem", Name: label, Bounds: bounds, Expanded: branch && t.IsBranchOpen(uid), Selected: binding.selected == uid, Actions: []string{"focus", "select", "click"}}
		if branch {
			node.Actions = append(node.Actions, "expand", "collapse")
		}
		s.nodes = append(s.nodes, node)
		if s.window.Canvas().Focused() == focusObject && binding.selected == uid {
			s.focus = id
		}
		u := uid
		s.actions[id] = func(a nativea11y.Action) {
			switch a.Kind {
			case "focus", "select", "click":
				s.window.Canvas().Focus(focusObject)
				t.Select(u)
			case "expand":
				t.OpenBranch(u)
			case "collapse":
				t.CloseBranch(u)
			}
		}
		if branch && t.IsBranchOpen(uid) && t.ChildUIDs != nil {
			for _, child := range t.ChildUIDs(uid) {
				walk(child, id)
			}
		}
	}
	if t.Root != "" {
		walk(t.Root, n.ID)
	} else if t.ChildUIDs != nil {
		for _, uid := range t.ChildUIDs(t.Root) {
			walk(uid, n.ID)
		}
	}
}
func (s *semanticWindow) snapshot() nativea11y.Snapshot {
	s.nodes = nil
	s.actions = make(map[uint64]func(nativea11y.Action))
	s.focus = 1
	size := s.window.Canvas().Size()
	s.nodes = append(s.nodes, nativea11y.Node{ID: 1, Role: "window", Name: s.window.Title(), Bounds: nativea11y.Bounds{Width: float64(size.Width), Height: float64(size.Height)}})
	overlays := s.window.Canvas().Overlays().List()
	if len(overlays) > 0 {
		s.visit(overlays[len(overlays)-1], 1, "")
	} else {
		s.visit(s.window.Content(), 1, "")
	}
	return nativea11y.Snapshot{Nodes: s.nodes, Focus: s.focus, WindowFocused: true}
}
func (s *semanticWindow) perform(a nativea11y.Action) {
	// Native events can queue between updates. Rebuild from the currently exposed
	// modal/control tree before accepting any action from an older snapshot.
	current := s.snapshot()
	for _, node := range current.Nodes {
		if node.ID == a.NodeID && !node.Disabled {
			for _, kind := range node.Actions {
				if kind == a.Kind {
					if f := s.actions[a.NodeID]; f != nil {
						f(a)
					}
					return
				}
			}
		}
	}
}

// The returned cleanup must run from the window's early OnClosed callback,
// before the native handle is destroyed. Context cancellation is a synchronous
// fallback and never relies on an application event queue that may have stopped.
func attachWindowAccessibility(ctx context.Context, w fyne.Window) func() {
	model := newSemanticWindow(w)
	var handle atomic.Pointer[nativea11y.Adapter]
	var closed atomic.Bool
	reported := false
	started := time.Now()
	cleanup := func() {
		closed.Store(true)
		if a := handle.Swap(nil); a != nil {
			a.Close()
		}
	}
	refresh := func() {
		if closed.Load() || ctx.Err() != nil {
			return
		}
		a := handle.Load()
		if a == nil {
			var err error
			a, err = nativea11y.Attach(w, func(action nativea11y.Action) {
				if !closed.Load() && ctx.Err() == nil {
					model.perform(action)
				}
			})
			if err != nil {
				if !reported && time.Since(started) > 5*time.Second {
					log.Printf("Native accessibility unavailable: %v", err)
					reported = true
				}
				return
			}
			if closed.Load() || ctx.Err() != nil {
				a.Close()
				return
			}
			handle.Store(a)
			if closed.Load() {
				if pending := handle.Swap(nil); pending != nil {
					pending.Close()
				}
				return
			}
		}
		// Native activation and backing scale can change independently of the tree.
		if err := a.Update(model.snapshot()); err != nil {
			log.Printf("Native accessibility update failed: %v", err)
		}
	}
	go func() { <-ctx.Done(); cleanup() }()
	startUIPoller(ctx, 200*time.Millisecond, fyne.Do, refresh)
	fyne.Do(refresh)
	return cleanup
}
