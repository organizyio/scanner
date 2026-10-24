//go:build cgo

package gui

import (
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"scan-projects/internal/nativea11y"
	"strings"
	"testing"
)

func TestAccessibleUnicodeSelectionRoundTrip(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	entry := widget.NewEntry()
	entry.SetText("aé🙂z")
	changes := 0
	entry.OnChanged = func(string) { changes++ }
	for _, selection := range []nativea11y.TextSelection{{Anchor: 1, Focus: 3}, {Anchor: 3, Focus: 1}, {Anchor: 2, Focus: 2}, {Anchor: 0, Focus: 4}, {Anchor: 4, Focus: 4}} {
		if !setEntrySelection(entry, &selection) {
			t.Fatal("valid selection rejected")
		}
		got := entrySelection(entry)
		if got == nil || *got != selection {
			t.Fatalf("got %+v want %+v selected %q", got, selection, entry.SelectedText())
		}
	}
	if changes != 0 || entry.Text != "aé🙂z" {
		t.Fatal("selection edited field or emitted value callback")
	}
	before := entry.CursorTextOffset()
	if setEntrySelection(entry, &nativea11y.TextSelection{Anchor: -1, Focus: 0}) || setEntrySelection(entry, &nativea11y.TextSelection{Anchor: 0, Focus: 5}) || entry.CursorTextOffset() != before {
		t.Fatal("invalid range changed field")
	}
	entry.SetText("a\r\nb")
	if entrySupportsSelection(entry) {
		t.Fatal("CRLF mapping not qualified")
	}
	entry.SetText(strings.Repeat("x", maxAccessibleEntryRunes+1))
	if entrySupportsSelection(entry) {
		t.Fatal("unbounded entry selection advertised")
	}
}
func TestAmbiguousSelectionIsNotInvented(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	entry := widget.NewEntry()
	entry.SetText("aaaa")
	if !setEntrySelection(entry, &nativea11y.TextSelection{Anchor: 1, Focus: 2}) {
		t.Fatal("selection failed")
	}
	if entrySelection(entry) != nil {
		t.Fatal("ambiguous anchor invented")
	}
}
