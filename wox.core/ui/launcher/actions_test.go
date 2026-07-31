package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestActionPanelEntryForHotkeyMatchesSelectedResultAction(t *testing.T) {
	results := []queryResult{{ID: "selected", Actions: []resultAction{{ID: "delete", Hotkey: "cmd+d"}}}}
	entries := unifiedActionPanelEntries(results, 0, nil)

	entry, matched := actionPanelEntryForHotkey(entries, woxui.KeyEvent{Key: "d", Modifiers: woxui.KeyModifierMeta, Down: true})
	if !matched {
		t.Fatal("Cmd+D did not match the selected result action")
	}
	if entry.Source != actionPanelSourceResult || entry.ResultIndex != 0 || entry.ActionIndex != 0 || entry.ID != "result-delete-0" {
		t.Fatalf("matched entry = %+v, want selected result delete action", entry)
	}
}

func TestActionPanelEntryForHotkeyKeepsToolbarPriority(t *testing.T) {
	results := []queryResult{{ID: "selected", Actions: []resultAction{{ID: "result-delete", Hotkey: "cmd+d"}}}}
	message := &toolbarMessage{ID: "message", Actions: []toolbarMessageAction{{ID: "toolbar-delete", Hotkey: "cmd+d"}}}
	entries := unifiedActionPanelEntries(results, 0, message)

	entry, matched := actionPanelEntryForHotkey(entries, woxui.KeyEvent{Key: "d", Modifiers: woxui.KeyModifierMeta, Down: true})
	if !matched {
		t.Fatal("Cmd+D did not match a unified action")
	}
	if entry.Source != actionPanelSourceToolbar || entry.ToolbarMessageAction.ID != "toolbar-delete" {
		t.Fatalf("matched entry = %+v, want toolbar action", entry)
	}
}

func TestActionPanelEntryForHotkeyIgnoresKeyUp(t *testing.T) {
	entries := []actionPanelEntry{{ID: "result-delete-0", Hotkey: "cmd+d", Source: actionPanelSourceResult}}
	if _, matched := actionPanelEntryForHotkey(entries, woxui.KeyEvent{Key: "d", Modifiers: woxui.KeyModifierMeta}); matched {
		t.Fatal("key-up unexpectedly matched Cmd+D")
	}
}

func TestOnResultActionHotkeyHandlesClosedPanel(t *testing.T) {
	app := &App{selected: 0, results: []queryResult{{ID: "selected", Actions: []resultAction{{ID: "delete", Type: "local", Hotkey: "cmd+d"}}}}}
	if !app.onResultActionHotkey(woxui.KeyEvent{Key: "d", Modifiers: woxui.KeyModifierMeta, Down: true}) {
		t.Fatal("closed action panel did not handle Cmd+D")
	}
}

func TestOnResultActionHotkeyLeavesDefaultEnterToLauncher(t *testing.T) {
	app := &App{selected: 0, results: []queryResult{{ID: "selected", Actions: []resultAction{{ID: "default", Type: "local", Hotkey: "enter"}}}}}
	if app.onResultActionHotkey(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}) {
		t.Fatal("result hotkey intercepted the launcher's default Enter handling")
	}
}
