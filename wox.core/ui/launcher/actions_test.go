package launcher

import (
	"runtime"
	"testing"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
)

func TestWebViewLocalActionPanelEntries(t *testing.T) {
	results := []queryResult{{ID: "webview", Preview: queryPreview{PreviewType: "webview"}}}
	entries := webViewLocalActionPanelEntries(results, 0, "windows")
	if len(entries) != 2 {
		t.Fatalf("webview local actions = %d, want 2", len(entries))
	}
	if entries[0].ID != localActionWebViewReloadID || entries[0].Hotkey != primaryHotkey("r") {
		t.Fatalf("reload action = %+v", entries[0])
	}
	if entries[1].ID != localActionWebViewOpenDevToolsID || entries[1].Hotkey != "" {
		t.Fatalf("developer tools action = %+v", entries[1])
	}
	if unsupported := webViewLocalActionPanelEntries(results, 0, "linux"); len(unsupported) != 0 {
		t.Fatalf("linux webview local actions = %d, want 0", len(unsupported))
	}
}

func TestUnifiedActionsReserveWebViewReloadHotkey(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("webview local actions are available on macOS and Windows")
	}
	results := []queryResult{{
		ID: "webview", Preview: queryPreview{PreviewType: "webview"},
		Actions: []resultAction{{ID: "plugin-reload", Hotkey: primaryHotkey("r")}},
	}}
	entries := unifiedActionPanelEntries(results, 0, nil)
	if len(entries) != 3 || entries[0].ID != localActionWebViewReloadID || entries[2].Hotkey != "" {
		t.Fatalf("unified webview actions = %+v", entries)
	}
}

func TestToolbarActionEntriesIncludesShortcutLocalActions(t *testing.T) {
	entries := []actionPanelEntry{
		{ID: localActionWebViewReloadID, Hotkey: "control+r", Source: actionPanelSourceLocal},
		{ID: localActionWebViewOpenDevToolsID, Source: actionPanelSourceLocal},
		{ID: "open", Hotkey: "enter", IsDefault: true, Source: actionPanelSourceResult},
		{ID: "folder", Hotkey: "control+enter", Source: actionPanelSourceResult},
		{ID: "message", Hotkey: "control+m", Source: actionPanelSourceToolbar},
	}
	withoutMessage := toolbarActionEntries(entries, false)
	if len(withoutMessage) != 3 || withoutMessage[0].ID != localActionWebViewReloadID || withoutMessage[1].ID != "open" || withoutMessage[2].ID != "folder" {
		t.Fatalf("toolbar actions without message = %+v", withoutMessage)
	}
	withMessage := toolbarActionEntries(entries, true)
	if len(withMessage) != 4 || withMessage[0].ID != "open" || withMessage[1].ID != "folder" || withMessage[2].ID != localActionWebViewReloadID || withMessage[3].ID != "message" {
		t.Fatalf("toolbar actions with message = %+v, want result, local, then message shortcuts", withMessage)
	}
}

func TestToolbarPinnedActionKeepsDefaultEnter(t *testing.T) {
	if !toolbarPinnedAction(actionPanelEntry{ID: "open", Hotkey: "enter", IsDefault: true}) {
		t.Fatal("default Enter should stay pinned on the footer")
	}
	if toolbarPinnedAction(actionPanelEntry{ID: "folder", Hotkey: "control+enter"}) {
		t.Fatal("secondary hotkey actions should yield to leftover width")
	}
}

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

func TestOnActionKeyIgnoresKeyUp(t *testing.T) {
	app := &App{actionPanel: true}
	if app.onActionKey(woxui.KeyEvent{Key: woxui.KeyArrowDown}) {
		t.Fatal("action panel handled key-up")
	}
}

func TestUnifiedActionPanelEntriesOrdersPluginThenSystem(t *testing.T) {
	results := []queryResult{{
		ID: "emoji",
		Actions: []resultAction{
			{ID: "__system_pin_in_query__", Name: "Pin", IsSystemAction: true},
			{ID: "copy", Name: "Copy"},
			{ID: "__system_reset_ranking__", Name: "Reset", IsSystemAction: true},
			{ID: "keyword", Name: "Add keyword"},
		},
	}}
	entries := unifiedActionPanelEntries(results, 0, nil)
	if len(entries) != 4 {
		t.Fatalf("unified entries = %d, want 4", len(entries))
	}
	got := make([]string, len(entries))
	for index, entry := range entries {
		got[index] = entry.ID
		if (entry.ID == "result-copy-1" || entry.ID == "result-keyword-3") && entry.IsSystemAction {
			t.Fatalf("plugin entry %s marked as system", entry.ID)
		}
		if (entry.ID == "result-__system_pin_in_query__-0" || entry.ID == "result-__system_reset_ranking__-2") && !entry.IsSystemAction {
			t.Fatalf("system entry %s missing system flag", entry.ID)
		}
	}
	want := []string{"result-copy-1", "result-keyword-3", "result-__system_pin_in_query__-0", "result-__system_reset_ranking__-2"}
	for index, id := range want {
		if got[index] != id {
			t.Fatalf("unified order = %v, want %v", got, want)
		}
	}
	if entries[0].ActionIndex != 1 || entries[2].ActionIndex != 0 {
		t.Fatalf("action indices = %+v, want original result.Actions positions", entries)
	}
}

func TestActionPanelDisplayItemsInsertsSeparatorWhenBothGroupsVisible(t *testing.T) {
	entries := []actionPanelEntry{
		{ID: "copy", IsSystemAction: false},
		{ID: "keyword", IsSystemAction: false},
		{ID: "pin", IsSystemAction: true},
	}
	items := actionPanelDisplayItems(entries, []int{0, 1, 2}, nil)
	if len(items) != 4 || items[2].Kind != launcherview.ActionItemKindSeparator {
		t.Fatalf("grouped items = %+v, want two plugin rows, a separator, then system", items)
	}
	if items[0].ID != "copy" || items[1].ID != "keyword" || items[3].ID != "pin" {
		t.Fatalf("grouped item ids = %+v", items)
	}
}

func TestActionPanelDisplayItemsOmitsSeparatorWhenOnlyOneGroup(t *testing.T) {
	pluginOnly := actionPanelDisplayItems([]actionPanelEntry{{ID: "copy"}, {ID: "keyword"}}, []int{0, 1}, nil)
	if len(pluginOnly) != 2 || pluginOnly[0].Kind != launcherview.ActionItemKindAction || pluginOnly[1].Kind != launcherview.ActionItemKindAction {
		t.Fatalf("plugin-only items = %+v, want a flat list", pluginOnly)
	}
	systemOnly := actionPanelDisplayItems([]actionPanelEntry{{ID: "pin", IsSystemAction: true}}, []int{0}, nil)
	if len(systemOnly) != 1 || systemOnly[0].Kind != launcherview.ActionItemKindAction {
		t.Fatalf("system-only items = %+v, want a flat list", systemOnly)
	}
}

func TestActionPanelDisplayItemsOmitsSeparatorWhenFilterLeavesOneGroup(t *testing.T) {
	entries := []actionPanelEntry{{ID: "copy"}, {ID: "pin", IsSystemAction: true}}
	items := actionPanelDisplayItems(entries, []int{1}, nil)
	if len(items) != 1 || items[0].ID != "pin" || items[0].Kind != launcherview.ActionItemKindAction {
		t.Fatalf("filtered system items = %+v, want no separator", items)
	}
}

func TestFilteredActionIndicesMatchNameAndAliases(t *testing.T) {
	actions := []actionPanelEntry{{Name: "复制路径", SearchAliases: []string{"Copy Path", "文件地址"}}}
	if matches := filteredActionIndices(actions, "复制", nil, true); len(matches) != 1 {
		t.Fatalf("localized action matches = %v, want [0]", matches)
	}
	if matches := filteredActionIndices(actions, "copy", nil, true); len(matches) != 1 {
		t.Fatalf("English action matches = %v, want [0]", matches)
	}
	if matches := filteredActionIndices(actions, "文件地址", nil, true); len(matches) != 1 {
		t.Fatalf("custom alias matches = %v, want [0]", matches)
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
