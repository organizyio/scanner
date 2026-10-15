package nativea11y

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSnapshotWireNamesAndNilActions(t *testing.T) {
	bytes, err := json.Marshal(Snapshot{Nodes: []Node{{ID: 1, Role: "window", Bounds: Bounds{Width: 50}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"id":1`, `"role":"window"`, `"Width":50`, `"window_focused":false`} {
		if !strings.Contains(string(bytes), expected) {
			t.Fatal(string(bytes))
		}
	}
	if strings.Contains(string(bytes), `"actions":null`) {
		t.Fatal("empty action list must omit rather than encode null")
	}
}

func TestClosedAdapterFailsAndCloseIsIdempotent(t *testing.T) {
	adapter := &Adapter{}
	adapter.Close()
	adapter.Close()
	if adapter.Update(Snapshot{}) == nil {
		t.Fatal("closed adapter must reject updates")
	}
}

func TestLogicalBoundsIncludeNativeBackingScale(t *testing.T) {
	original := Snapshot{Nodes: []Node{{ID: 1, Bounds: Bounds{X: 10, Y: 20, Width: 100, Height: 40}}}}
	for _, backing := range []float64{1, 2} {
		result := scaledSnapshot(original, 1*backing)
		if result.Nodes[0].Bounds.Width != 100*backing || result.Nodes[0].Bounds.X != 10*backing {
			t.Fatal(result)
		}
		// AccessKit macOS converts pixels back to points for AXFrame.
		if result.Nodes[0].Bounds.Width/backing != 100 {
			t.Fatal("incorrect native AX frame")
		}
	}
	if original.Nodes[0].Bounds.Width != 100 {
		t.Fatal("caller snapshot mutated")
	}
}
