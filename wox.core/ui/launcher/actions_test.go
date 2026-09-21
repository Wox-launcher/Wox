package launcher

import (
	"runtime"
	"testing"

	"wox/common/icons"
	"wox/setting"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type actionDamageServices struct {
	formTableHostServices
	damage woxui.Rect
	full   bool
}

func (s *actionDamageServices) Invalidate() error                    { s.full = true; return nil }
func (s *actionDamageServices) InvalidateRect(rect woxui.Rect) error { s.damage = rect; return nil }

// TestActionUpdatesRepaintOldAndNewPanelBounds covers typing, clearing, and selection without a native window.
func TestActionUpdatesRepaintOldAndNewPanelBounds(t *testing.T) {
	app := &App{generalSettings: &generalSettingsController{}, actionPanel: true, actionFilter: woxui.NewTextEditor(""), results: []queryResult{{Actions: []resultAction{{ID: "open", Name: "Open"}, {ID: "copy", Name: "Copy"}}}}}
	services := &actionDamageServices{}
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		height := float32(200)
		if app.actionFilter.State().Text != "" {
			height = 100
		}
		return woxwidget.Stack{Width: 800, Height: 600, Children: []woxwidget.StackChild{{
			Left: 450, Top: 550 - height,
			Child: woxwidget.Gesture{ID: "action-panel-surface", Child: woxwidget.Container{Width: 300, Height: height, Floating: true}},
		}, {Top: 570, Child: woxwidget.Container{Width: 800, Height: 30, Floating: true}}}}
	})
	app.host = host
	host.AttachServices(services)
	defer host.Dispose()
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 800, Height: 600}, Scale: 1}
	host.Frame(&woxui.DisplayList{}, frame)
	previousDamage := woxui.Rect{}
	for _, update := range []func(){func() { app.setActionFilterValue("missing") }, func() { app.setActionFilterValue("") }, func() { app.moveActionSelection(1) }, func() { app.selectAction(0) }} {
		services.damage, services.full = woxui.Rect{}, false
		update()
		if services.full || services.damage.Width <= 0 {
			t.Fatalf("action update requested full or no repaint: %+v", services)
		}
		frame.Damage = services.damage
		// The native two-buffer surface also restores the previous frame's damage.
		if previousDamage.Width > 0 {
			top := min(previousDamage.Y, frame.Damage.Y)
			frame.Damage.Y, frame.Damage.Height = top, 550-top
		}
		previousDamage = services.damage
		var list woxui.DisplayList
		host.Frame(&list, frame)
		want := woxui.Rect{X: 446, Y: 346, Width: 308, Height: 208}
		if len(list.RenderedFloatingMaterialRects()) > 0 {
			margin := woxui.FloatingMaterialBlurMargin
			want = woxui.Rect{X: want.X - margin, Y: want.Y - margin, Width: want.Width + 2*margin, Height: want.Height + 2*margin}
		}
		if got := list.NativeDamage(); got != want {
			t.Fatalf("action damage = %+v, want old and new panel bounds %+v", got, want)
		}
		// A user can pause before clearing. Let both buffers forget the larger panel.
		if app.actionFilter.State().Text != "" {
			frame.Damage = services.damage
			frame.Damage.Y, frame.Damage.Height = 450, 100
			previousDamage = frame.Damage
			for range 3 {
				host.Frame(&woxui.DisplayList{}, frame)
			}
		}
	}
}

func TestAboutMenuIconsUseMonochromeArtwork(t *testing.T) {
	for _, entry := range aboutMenuEntries("") {
		if entry.IsGroupHeader {
			continue
		}
		if entry.ID == aboutMenuFeedbackID || entry.ID == aboutMenuGithubID || entry.ID == aboutMenuRedditID || entry.ID == aboutMenuDiscordID {
			if !svgUsesThemeIconColor(entry.Icon) {
				t.Fatalf("%s needs theme-adaptive monochrome artwork", entry.ID)
			}
		}
	}
}

func TestActionPanelTintsThemeAdaptiveSVGOnly(t *testing.T) {
	if !svgUsesThemeIconColor(fromCoreImage(icons.Get(icons.ActionCopy))) {
		t.Fatal("action.copy must follow the row text tint")
	}
	if svgUsesThemeIconColor(fromCoreImage(icons.Get(icons.PluginApp))) {
		t.Fatal("plugin.app is a brand SVG and must not be flattened by a source-in tint")
	}
	if svgUsesThemeIconColor(settingControlIconSource("refresh")) {
		t.Fatal("control masks do not use the theme variable; local actions tint them separately")
	}
}

func TestWebViewLocalActionPanelEntries(t *testing.T) {
	results := []queryResult{{ID: "webview", Preview: queryPreview{PreviewType: "webview"}}}
	entries := webViewLocalActionPanelEntries(results, 0, "windows", false)
	if len(entries) != 6 {
		t.Fatalf("webview local actions = %d, want 6", len(entries))
	}
	if entries[0].ID != localActionWebViewReloadID || entries[0].Hotkey != primaryHotkey("r") {
		t.Fatalf("reload action = %+v", entries[0])
	}
	if entries[1].ID != localActionWebViewGoBackID || entries[1].Hotkey != primaryHotkey("[") {
		t.Fatalf("back action = %+v", entries[1])
	}
	if entries[2].ID != localActionWebViewGoForwardID || entries[2].Hotkey != primaryHotkey("]") {
		t.Fatalf("forward action = %+v", entries[2])
	}
	if entries[3].ID != localActionWebViewOpenInBrowserID || entries[3].Hotkey != primaryHotkey("o") {
		t.Fatalf("open in browser action = %+v", entries[3])
	}
	if entries[4].ID != localActionWebViewClosePageID || entries[4].Hotkey != primaryHotkey("w") {
		t.Fatalf("close page action = %+v", entries[4])
	}
	if entries[5].ID != localActionWebViewOpenDevToolsID || entries[5].Hotkey != "" {
		t.Fatalf("developer tools action = %+v", entries[5])
	}
	withHide := webViewLocalActionPanelEntries(results, 0, "windows", true)
	if len(withHide) != 7 || withHide[4].ID != localActionWebViewHidePageID || withHide[4].Hotkey != primaryHotkey("h") || withHide[5].ID != localActionWebViewClosePageID {
		t.Fatalf("hide page action = %+v", withHide)
	}
	if unsupported := webViewLocalActionPanelEntries(results, 0, "linux", false); len(unsupported) != 0 {
		t.Fatalf("linux webview local actions = %d, want 0", len(unsupported))
	}
}

func TestActionPanelEntryForHotkeyOpensWebViewInBrowser(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("webview local actions are available on macOS and Windows")
	}
	results := []queryResult{{ID: "webview", Preview: queryPreview{PreviewType: "webview"}}}
	entries := unifiedActionPanelEntries(results, 0, nil)
	modifiers := woxui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		modifiers = woxui.KeyModifierMeta
	}
	entry, matched := actionPanelEntryForHotkey(entries, woxui.KeyEvent{Key: "o", Modifiers: modifiers, Down: true})
	if !matched || entry.ID != localActionWebViewOpenInBrowserID {
		t.Fatalf("Ctrl/Cmd+O = matched=%v entry=%+v, want open in browser", matched, entry)
	}
}

func TestActionPanelEntryForHotkeyClosesWebViewPage(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("webview local actions are available on macOS and Windows")
	}
	results := []queryResult{{ID: "webview", Preview: queryPreview{PreviewType: "webview"}}}
	entries := unifiedActionPanelEntries(results, 0, nil)
	modifiers := woxui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		modifiers = woxui.KeyModifierMeta
	}
	entry, matched := actionPanelEntryForHotkey(entries, woxui.KeyEvent{Key: "w", Modifiers: modifiers, Down: true})
	if !matched || entry.ID != localActionWebViewClosePageID {
		t.Fatalf("Ctrl/Cmd+W = matched=%v entry=%+v, want close webpage", matched, entry)
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
	if len(entries) != 7 || entries[0].ID != localActionWebViewReloadID || entries[6].Hotkey != "" {
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

func TestActionPanelHotkeyUsesConfiguredShortcut(t *testing.T) {
	app := &App{generalSettings: newGeneralSettingsController(CommonDeps{}, newSharedEditState())}
	if got := app.actionPanelHotkey(); got != setting.DefaultActionPanelHotkey() {
		t.Fatalf("empty setting = %q, want default %q", got, setting.DefaultActionPanelHotkey())
	}
	app.generalSettings.ApplyData(settingsData{ActionPanelHotkey: setting.LegacyActionPanelHotkey()})
	if got := app.actionPanelHotkey(); got != setting.LegacyActionPanelHotkey() {
		t.Fatalf("configured setting = %q, want legacy %q", got, setting.LegacyActionPanelHotkey())
	}
	event := woxui.KeyEvent{Key: "j", Modifiers: woxui.KeyModifierControl, Down: true}
	if runtime.GOOS == "darwin" {
		event.Modifiers = woxui.KeyModifierMeta
	}
	if !hotkeyMatches(app.actionPanelHotkey(), event) {
		t.Fatal("configured primary+J should still toggle the action panel")
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
			{ID: "__system_reset_ranking__", Name: "Reset", Tail: "+55", IsSystemAction: true},
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
	if entries[3].Tail != "+55" {
		t.Fatalf("reset ranking tail = %q, want +55", entries[3].Tail)
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

func TestActionPanelWindowHeightIgnoresFilterWhenGroupsAppear(t *testing.T) {
	entries := []actionPanelEntry{
		{ID: "paste"}, {ID: "copy"}, {ID: "save"}, {ID: "pin", IsSystemAction: true}, {ID: "reset", IsSystemAction: true},
	}
	unfiltered := actionPanelVisibleListHeight(entries, actionPanelUnfilteredIndices(entries))
	filtered := actionPanelVisibleListHeight(entries, []int{0, 3})
	if unfiltered <= filtered {
		t.Fatalf("unfiltered height %v should keep the group divider and stay taller than filtered %v", unfiltered, filtered)
	}
	if unfiltered != float32(3*launcherview.ActionRowHeight+launcherview.ActionGroupDividerHeight+2*launcherview.ActionRowHeight) {
		t.Fatalf("unfiltered grouped height = %v, want three plugin rows, a divider, and two system rows", unfiltered)
	}
}

func TestActionPanelFloatingPlacementCanAnchorLeft(t *testing.T) {
	slot, occlusion := actionPanelFloatingPlacement(18, 600, 80, 40, 320, 400, 20)
	if slot.Left != 18 || occlusion.X != 18 {
		t.Fatalf("left about-menu placement = slot %+v occlusion %+v, want X 18", slot, occlusion)
	}
}

func TestActionPanelFloatingPlacementKeepsSearchPinnedWhenListShrinks(t *testing.T) {
	bottomOffset := launcherview.ActionPanelBottomOffset(10)
	full, _ := actionPanelFloatingPlacement(20, 600, 80, 40, 320, 400, bottomOffset)
	filtered, _ := actionPanelFloatingPlacement(20, 600, 80, 40, 320, 280, bottomOffset)
	if !full.AnchorBottom || !filtered.AnchorBottom {
		t.Fatal("action panel must be bottom-anchored so the search box stays put")
	}
	if full.Bottom != filtered.Bottom || full.Bottom != 40+bottomOffset {
		t.Fatalf("search-box bottom = %v / %v, want a stable toolbar inset %v", full.Bottom, filtered.Bottom, 40+bottomOffset)
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

func TestFilteredActionIndicesRanksPrefixAboveScatteredMatch(t *testing.T) {
	actions := []actionPanelEntry{{Name: "Run as Administrator"}, {Name: "Uninstall"}}
	matches := filteredActionIndices(actions, "uninsta", nil, false)
	if len(matches) < 1 || matches[0] != 1 {
		t.Fatalf("filtered order = %v, want Uninstall first", matches)
	}
}

func TestFilteredActionIndicesEmptyQueryKeepsSourceOrder(t *testing.T) {
	actions := []actionPanelEntry{{Name: "Run as Administrator"}, {Name: "Uninstall"}}
	matches := filteredActionIndices(actions, "", nil, false)
	if len(matches) != 2 || matches[0] != 0 || matches[1] != 1 {
		t.Fatalf("empty filter order = %v, want source order", matches)
	}
}

func TestFilteredActionIndicesKeepsSystemActionsAfterPluginMatches(t *testing.T) {
	actions := []actionPanelEntry{{Name: "Pin", IsSystemAction: true}, {Name: "Uninstall"}}
	matches := filteredActionIndices(actions, "in", nil, false)
	if len(matches) != 2 || matches[0] != 1 || matches[1] != 0 {
		t.Fatalf("grouped filter order = %v, want plugin then system", matches)
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
