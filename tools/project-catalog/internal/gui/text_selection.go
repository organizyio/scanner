//go:build cgo

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"scan-projects/internal/nativea11y"
	"strings"
	"unicode/utf8"
)

// Use public Fyne entry APIs only. Multiline/CRLF and very large fields need a
// separate mapping; never advertise a range action we cannot apply faithfully.
const maxAccessibleEntryRunes = 4096

func entrySupportsSelection(e *widget.Entry) bool {
	return !e.MultiLine && !e.Password && !strings.ContainsAny(e.Text, "\r\n") && utf8.RuneCountInString(e.Text) <= maxAccessibleEntryRunes
}
func entrySelection(e *widget.Entry) *nativea11y.TextSelection {
	if !entrySupportsSelection(e) {
		return nil
	}
	text := []rune(e.Text)
	focus := e.CursorTextOffset()
	if focus < 0 || focus > len(text) {
		return nil
	}
	selected := e.SelectedText()
	if selected == "" {
		return &nativea11y.TextSelection{Anchor: focus, Focus: focus}
	}
	width := utf8.RuneCountInString(selected)
	var candidates []int
	if focus >= width && string(text[focus-width:focus]) == selected {
		candidates = append(candidates, focus-width)
	}
	if focus+width <= len(text) && string(text[focus:focus+width]) == selected {
		candidates = append(candidates, focus+width)
	}
	if len(candidates) == 1 {
		return &nativea11y.TextSelection{Anchor: candidates[0], Focus: focus}
	}
	// Fyne exposes selected text but not its anchor. Repeated text can make the
	// direction ambiguous; omission is preferable to fabricating a range.
	return nil
}
func setEntrySelection(e *widget.Entry, selection *nativea11y.TextSelection) bool {
	if selection == nil || !entrySupportsSelection(e) {
		return false
	}
	length := utf8.RuneCountInString(e.Text)
	if selection.Anchor < 0 || selection.Focus < 0 || selection.Anchor > length || selection.Focus > length {
		return false
	}
	e.KeyUp(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	e.KeyUp(&fyne.KeyEvent{Name: desktop.KeyShiftRight})
	e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyHome})
	for i := 0; i < selection.Anchor; i++ {
		e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	}
	e.KeyDown(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	key, count := fyne.KeyRight, selection.Focus-selection.Anchor
	if count < 0 {
		key, count = fyne.KeyLeft, -count
	}
	for i := 0; i < count; i++ {
		e.TypedKey(&fyne.KeyEvent{Name: key})
	}
	e.KeyUp(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	e.Refresh()
	return true
}
