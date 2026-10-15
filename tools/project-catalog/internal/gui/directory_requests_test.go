package gui

import "testing"

func TestEditingPathCannotApplyAnOlderDirectoryResult(t *testing.T) {
	state := directoryRequests{}
	typedPath := "/synthetic/old"
	currentPath := ""
	selectEnabled := false
	pending := state.begin()
	// Editing disables selection immediately while the original ReadDir is pending.
	typedPath = "/synthetic/new"
	state.invalidate()
	selectEnabled = false
	applied := state.apply(pending, func() { currentPath = "/synthetic/old"; selectEnabled = true })
	if applied || selectEnabled || currentPath != "" || typedPath != "/synthetic/new" {
		t.Fatal("stale directory result enabled an unrelated selection")
	}
	// Returning to the same text still invalidates the earlier request.
	typedPath = "/synthetic/old"
	state.invalidate()
	if state.apply(pending, func() { selectEnabled = true }) {
		t.Fatal("returning to old text revived a stale request")
	}
	latest := state.begin()
	if !state.apply(latest, func() { currentPath = typedPath; selectEnabled = true }) || !selectEnabled || currentPath != typedPath {
		t.Fatal("current directory result not applied")
	}
}
