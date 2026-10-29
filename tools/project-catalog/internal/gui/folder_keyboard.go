//go:build cgo

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"image/color"
	"math"
)

// FolderTree keeps visual selection and native semantic focus on the same row.
// Navigation uses only public Tree APIs, never the private renderer focus state.
type FolderTree struct {
	*widget.Tree
	selected      string
	focused       bool
	semanticLabel func(string) string
}

func newFolderTree(tree *widget.Tree) *FolderTree {
	// This constructor adopts a fresh, not-yet-rendered Tree. Rebind its public
	// BaseWidget before first use so pointer and keyboard focus target the wrapper.
	tree.BaseWidget = widget.BaseWidget{}
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
	t.focused = true
	t.Tree.FocusGained()
	t.Refresh()
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

func (t *FolderTree) FocusLost() { t.focused = false; t.Tree.FocusLost(); t.Refresh() }

// Keep the original tree renderer's layout, scrolling and lifecycle, adding only
// a visible focus boundary independent of the retained selected-row styling.
func (t *FolderTree) CreateRenderer() fyne.WidgetRenderer {
	inner := t.Tree.CreateRenderer()
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeWidth = 3
	r := &folderTreeRenderer{inner: inner, tree: t, border: border}
	r.refreshBorder()
	return r
}

type folderTreeRenderer struct {
	inner  fyne.WidgetRenderer
	tree   *FolderTree
	border *canvas.Rectangle
}

func (r *folderTreeRenderer) Destroy()           { r.inner.Destroy() }
func (r *folderTreeRenderer) MinSize() fyne.Size { return r.inner.MinSize() }
func (r *folderTreeRenderer) Objects() []fyne.CanvasObject {
	return append(append([]fyne.CanvasObject(nil), r.inner.Objects()...), r.border)
}
func (r *folderTreeRenderer) Layout(size fyne.Size) {
	r.inner.Layout(size)
	inset := r.border.StrokeWidth / 2
	r.border.Move(fyne.NewPos(inset, inset))
	r.border.Resize(fyne.NewSize(max(float32(0), size.Width-2*inset), max(float32(0), size.Height-2*inset)))
}
func (r *folderTreeRenderer) Refresh() {
	r.inner.Refresh()
	r.refreshBorder()
	r.Layout(r.tree.Size())
	canvas.Refresh(r.tree)
}
func (r *folderTreeRenderer) refreshBorder() {
	accent := color.NRGBAModel.Convert(theme.Color(theme.ColorNameFocus)).(color.NRGBA)
	accent.A = 255
	background := theme.Color(theme.ColorNameBackground)
	if colorContrast(accent, background) < 3 {
		accent = color.NRGBAModel.Convert(theme.Color(theme.ColorNameForeground)).(color.NRGBA)
		accent.A = 255
	}
	r.border.StrokeColor = accent
	if r.tree.focused {
		r.border.Show()
	} else {
		r.border.Hide()
	}
	r.border.Refresh()
}
func colorContrast(a, b color.Color) float64 {
	luminance := func(c color.Color) float64 {
		v := color.NRGBAModel.Convert(c).(color.NRGBA)
		linear := func(channel uint8) float64 {
			x := float64(channel) / 255
			if x <= 0.04045 {
				return x / 12.92
			}
			return math.Pow((x+0.055)/1.055, 2.4)
		}
		return .2126*linear(v.R) + .7152*linear(v.G) + .0722*linear(v.B)
	}
	x, y := luminance(a), luminance(b)
	if x < y {
		x, y = y, x
	}
	return (x + .05) / (y + .05)
}
