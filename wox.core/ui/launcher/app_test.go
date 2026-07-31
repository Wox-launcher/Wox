package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestLauncherWindowOriginPreservesDraggedPosition(t *testing.T) {
	params := showAppParams{Position: position{X: 400, Y: 300}}
	current := woxui.Rect{X: 92, Y: 74, Width: 760, Height: 420}

	x, y := launcherWindowOrigin(params, current, 620, false)
	if x != current.X || y != current.Y {
		t.Fatalf("preserved origin = %.0f,%.0f, want %.0f,%.0f", x, y, current.X, current.Y)
	}
}

func TestLauncherWindowOriginKeepsBottomQueryBoxAnchored(t *testing.T) {
	params := showAppParams{QueryBoxAtBottom: true}
	current := woxui.Rect{X: 92, Y: 200, Width: 760, Height: 420}

	x, y := launcherWindowOrigin(params, current, 620, false)
	if x != current.X || y != 0 {
		t.Fatalf("bottom-anchored origin = %.0f,%.0f, want %.0f,0", x, y, current.X)
	}
}

func TestLauncherWindowOriginUsesShowPositionWhenRequested(t *testing.T) {
	params := showAppParams{Position: position{X: 400, Y: 300}}
	current := woxui.Rect{X: 92, Y: 74, Width: 760, Height: 420}

	x, y := launcherWindowOrigin(params, current, 620, true)
	if x != 400 || y != 300 {
		t.Fatalf("show origin = %.0f,%.0f, want 400,300", x, y)
	}
}

func TestSelectableIndexFromPreservesExplicitRefreshIndex(t *testing.T) {
	results := []queryResult{{ID: "first"}, {ID: "group", IsGroup: true}, {ID: "third"}}

	if index := selectableIndex(results); index != 0 {
		t.Fatalf("default selected index = %d, want 0", index)
	}
	if index := selectableIndexFrom(results, 1); index != 2 {
		t.Fatalf("preserved selected index = %d, want 2", index)
	}
}

func TestHotkeyMatchesOnlyKeyDown(t *testing.T) {
	event := woxui.KeyEvent{Key: "j", Modifiers: woxui.KeyModifierControl}
	if hotkeyMatches("ctrl+j", event) {
		t.Fatal("key-up unexpectedly matched Ctrl+J")
	}
	event.Down = true
	if !hotkeyMatches("ctrl+j", event) {
		t.Fatal("key-down did not match Ctrl+J")
	}
	if !hotkeyMatches("cmd+t", woxui.KeyEvent{Key: "t", Modifiers: woxui.KeyModifierMeta, Down: true}) {
		t.Fatal("key-down did not match Cmd+T")
	}
	if hotkeyMatches("cmd+t", woxui.KeyEvent{Key: "t", Modifiers: woxui.KeyModifierMeta, Down: true, Composing: true}) {
		t.Fatal("composing key unexpectedly matched Cmd+T")
	}
}
