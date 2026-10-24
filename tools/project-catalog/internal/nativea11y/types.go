// Package nativea11y exposes Scanner's semantic controls through native accessibility.
package nativea11y

import "fyne.io/fyne/v2"

// Bounds are logical content-relative coordinates; the adapter applies native scale.
type Bounds struct{ X, Y, Width, Height float64 }

// TextSelection uses Unicode scalar indexes, not UTF-8 byte or UTF-16 offsets.
type TextSelection struct {
	Anchor int `json:"anchor"`
	Focus  int `json:"focus"`
}
type Node struct {
	ID          uint64         `json:"id"`
	Parent      uint64         `json:"parent"`
	Role        string         `json:"role"`
	Name        string         `json:"name"`
	Value       string         `json:"value"`
	Description string         `json:"description,omitempty"`
	Placeholder string         `json:"placeholder,omitempty"`
	Bounds      Bounds         `json:"bounds"`
	Disabled    bool           `json:"disabled"`
	Checked     bool           `json:"checked"`
	Expanded    bool           `json:"expanded"`
	Selected    bool           `json:"selected"`
	Live        string         `json:"live,omitempty"`
	Actions     []string       `json:"actions,omitempty"`
	Selection   *TextSelection `json:"selection,omitempty"`
}
type Snapshot struct {
	Nodes         []Node `json:"nodes"`
	Focus         uint64 `json:"focus"`
	WindowFocused bool   `json:"window_focused"`
}
type Action struct {
	NodeID    uint64         `json:"node_id"`
	Kind      string         `json:"kind"`
	Value     string         `json:"value"`
	Selection *TextSelection `json:"selection,omitempty"`
}

// Attach obtains the native window after creation. Actions run asynchronously on
// the Fyne event loop. Call Close before releasing the native window.
func Attach(window fyne.Window, handler func(Action)) (*Adapter, error) {
	return attach(window, handler)
}

// scaledSnapshot converts Fyne logical content bounds to adapter physical pixels.
// Cocoa's adapter divides these by the window's native backing scale again.
func scaledSnapshot(snapshot Snapshot, scale float64) Snapshot {
	snapshot.Nodes = append([]Node(nil), snapshot.Nodes...)
	for i := range snapshot.Nodes {
		b := &snapshot.Nodes[i].Bounds
		b.X *= scale
		b.Y *= scale
		b.Width *= scale
		b.Height *= scale
	}
	return snapshot
}
