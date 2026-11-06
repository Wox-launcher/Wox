package widget

import (
	"testing"
	ui "wox/ui/runtime"
)

// TestShortcutScopeDoesNotAddFocusTargetsOrEscapeModals tests the infrastructure without WoxView.
func TestShortcutScopeDoesNotAddFocusTargetsOrEscapeModals(t *testing.T) {
	global, local := 0, 0
	host := NewHost(func(ui.FrameInfo) Widget {
		return ShortcutScope{Key: "root", Window: true, OnKey: func(ui.KeyEvent) bool { global++; return true }, Child: ShortcutScope{Key: "local", OnKey: func(ui.KeyEvent) bool { local++; return true }, Child: testButton("target", nil)}}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()
	renderTestFrame(host)
	e := ui.KeyEvent{Key: "x", Down: true}
	host.Key(e)
	if global != 1 || local != 0 || len(host.focusOrder()) != 1 {
		t.Fatal("no-focus shortcut routing or traversal changed")
	}
	host.RequestFocus("target")
	host.Key(e)
	if global != 1 || local != 1 {
		t.Fatal("nearest scope did not win")
	}
	host.SetOverlay("target", FocusScope{Key: "modal", Modal: true, Child: testButton("menu", nil)})
	renderTestFrame(host)
	host.Key(e)
	if global != 1 || local != 1 {
		t.Fatal("background shortcut fired through modal")
	}
	host.ClearOverlay("target", 0)
	renderTestFrame(host)
	if host.FocusedKey() != "target" {
		t.Fatalf("overlay did not restore stable focus: %s", host.FocusedKey())
	}
}
