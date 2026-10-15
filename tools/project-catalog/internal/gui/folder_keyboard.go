//go:build cgo

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// FolderTree keeps visual selection and native semantic focus on the same row.
// Navigation uses only public Tree APIs, never the private renderer focus state.
type FolderTree struct {
	*widget.Tree
	selected string
}

func newFolderTree(tree *widget.Tree) *FolderTree {
	wrapper := &FolderTree{Tree: tree}
	selected := tree.OnSelected
	tree.OnSelected = func(uid string) {
		wrapper.selected = uid
		if selected != nil {
			selected(uid)
		}
	}
	unselected := tree.OnUnselected
	tree.OnUnselected = func(uid string) {
		if wrapper.selected == uid {
			wrapper.selected = ""
		}
		if unselected != nil {
			unselected(uid)
		}
	}
	closed := tree.OnBranchClosed
	tree.OnBranchClosed = func(uid string) {
		if closed != nil {
			closed(uid)
		}
		if wrapper.selected != "" {
			visible, _ := wrapper.visibleRows()
			found := false
			for _, row := range visible {
				if row == wrapper.selected {
					found = true
					break
				}
			}
			if !found {
				tree.Select(uid)
			}
		}
	}
	tree.ExtendBaseWidget(wrapper)
	return wrapper
}

func (t *FolderTree) visibleRows() ([]string, map[string]string) {
	var rows []string
	parents := map[string]string{}
	seen := map[string]bool{}
	var visit func(string, string)
	visit = func(uid, parent string) {
		if seen[uid] {
			return
		}
		seen[uid] = true
		rows = append(rows, uid)
		parents[uid] = parent
		if t.IsBranch != nil && t.IsBranch(uid) && t.IsBranchOpen(uid) && t.ChildUIDs != nil {
			for _, child := range t.ChildUIDs(uid) {
				visit(child, uid)
			}
		}
	}
	if t.ChildUIDs != nil {
		for _, uid := range t.ChildUIDs(t.Root) {
			visit(uid, t.Root)
		}
	}
	return rows, parents
}

func (t *FolderTree) FocusGained() {
	t.Tree.FocusGained()
	rows, _ := t.visibleRows()
	if len(rows) == 0 {
		return
	}
	for _, uid := range rows {
		if uid == t.selected {
			return
		}
	}
	t.Select(rows[0])
}

func (t *FolderTree) TypedKey(event *fyne.KeyEvent) {
	rows, parents := t.visibleRows()
	if len(rows) == 0 {
		return
	}
	current := t.selected
	index := -1
	for i, uid := range rows {
		if uid == current {
			index = i
			break
		}
	}
	if index < 0 {
		index = 0
		current = rows[0]
	}
	target := current
	switch event.Name {
	case fyne.KeyDown:
		if index < len(rows)-1 {
			target = rows[index+1]
		}
	case fyne.KeyUp:
		if index > 0 {
			target = rows[index-1]
		}
	case fyne.KeyRight:
		if t.IsBranch != nil && t.IsBranch(current) {
			if !t.IsBranchOpen(current) {
				t.OpenBranch(current)
			} else if t.ChildUIDs != nil {
				if children := t.ChildUIDs(current); len(children) > 0 {
					target = children[0]
				}
			}
		}
	case fyne.KeyLeft:
		if t.IsBranch != nil && t.IsBranch(current) && t.IsBranchOpen(current) {
			t.CloseBranch(current)
		} else if parent := parents[current]; parent != t.Root && parent != "" {
			target = parent
		}
	case fyne.KeySpace, fyne.KeyReturn, fyne.KeyEnter:
	default:
		t.Tree.TypedKey(event)
		return
	}
	t.Select(target)
}
