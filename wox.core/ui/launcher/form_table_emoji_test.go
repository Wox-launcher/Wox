package launcher

import (
	"encoding/json"
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestOpenFormTableEmojiPickerTargetsIconField(t *testing.T) {
	app := trayQueryEditorTestApp(t)
	state := app.settingsTableEditor

	app.openFormTableEmojiPicker(0)
	if state.emojiPicker == nil {
		t.Fatal("emoji picker did not open for the icon field")
	}
	if state.emojiPicker.fieldIndex != 0 {
		t.Fatalf("emoji picker field index = %d, want 0", state.emojiPicker.fieldIndex)
	}
	if state.emojiPicker.initialEmoji != "📋" {
		t.Fatalf("emoji picker initial emoji = %q, want 📋", state.emojiPicker.initialEmoji)
	}
	if len(app.formTableEmojiSearchEntries) < 5000 {
		t.Fatalf("emoji picker search catalog count = %d, want at least 5000", len(app.formTableEmojiSearchEntries))
	}

	app.closeFormTableEmojiPicker()
	if state.emojiPicker != nil {
		t.Fatal("emoji picker should close")
	}

	app.openFormTableEmojiPicker(1)
	if state.emojiPicker != nil {
		t.Fatal("emoji picker must not open for a non-woxImage field")
	}
}

func TestChooseFormTableEmojiCommitsAndCloses(t *testing.T) {
	app := trayQueryEditorTestApp(t)
	state := app.settingsTableEditor
	app.openFormTableEmojiPicker(0)

	app.chooseFormTableEmoji("🤖")
	if state.emojiPicker != nil {
		t.Fatal("emoji picker should close after choosing")
	}
	if value := state.rowForm.values["Icon"]; value != "🤖" {
		t.Fatalf("icon after choosing = %q, want 🤖", value)
	}

	app.openFormTableEmojiPicker(0)
	app.chooseFormTableEmoji("")
	if state.emojiPicker == nil {
		t.Fatal("empty emoji should not commit")
	}
}

func TestWoxImageRowFieldIsNotTextEditable(t *testing.T) {
	if formDefinitionTextEditable(formDefinition{Type: "woxImage"}) {
		t.Fatal("woxImage fields must not be text-editable")
	}
}

func TestTrayQueryRowEditorFocusesQueryNotIcon(t *testing.T) {
	app := trayQueryEditorTestApp(t)
	state := app.settingsTableEditor
	if state.rowForm == nil {
		t.Fatal("tray query row editor did not open")
	}
	if state.rowForm.focused == 0 {
		t.Fatal("the icon field must not hold text focus")
	}
	if state.rowForm.editor == nil {
		t.Fatal("the query field should own the row editor text input")
	}
	if definition := state.rowForm.definitions[state.rowForm.focused]; definition.Value.Key != "Query" {
		t.Fatalf("focused row field = %q, want Query", definition.Value.Key)
	}
}

func TestFormTableEmojiCatalogIncludesExpandedGroups(t *testing.T) {
	groups := make(map[string]formTableEmojiGroup, len(formTableEmojiGroups))
	for _, group := range formTableEmojiGroups {
		groups[group.LabelKey] = group
		if group.Marker == "" {
			t.Fatalf("emoji group %q has no sidebar marker", group.LabelKey)
		}
	}
	if len(groups["ui_select_emoji_group_recommended"].Emojis) < 70 {
		t.Fatalf("recommended emoji count = %d, want at least 70", len(groups["ui_select_emoji_group_recommended"].Emojis))
	}
	for _, key := range []string{"ui_select_emoji_group_nature", "ui_select_emoji_group_flags"} {
		if len(groups[key].Emojis) < 40 {
			t.Fatalf("emoji group %q count = %d, want at least 40", key, len(groups[key].Emojis))
		}
	}
}

func TestRememberFormTableEmojiMaintainsUniqueMRU(t *testing.T) {
	app := &App{}
	for index := 0; index < 30; index++ {
		app.rememberFormTableEmoji(string(rune('A' + index)))
	}
	if len(app.recentFormTableEmojis) != 24 {
		t.Fatalf("recent emoji count = %d, want 24", len(app.recentFormTableEmojis))
	}
	app.rememberFormTableEmoji("Z")
	if app.recentFormTableEmojis[0] != "Z" || len(app.recentFormTableEmojis) != 24 {
		t.Fatalf("recent emojis after reuse = %v", app.recentFormTableEmojis)
	}
	seen := make(map[string]struct{}, len(app.recentFormTableEmojis))
	for _, emoji := range app.recentFormTableEmojis {
		if _, exists := seen[emoji]; exists {
			t.Fatalf("recent emojis contain duplicate %q", emoji)
		}
		seen[emoji] = struct{}{}
	}
}

func TestFormTableEmojiPickerSearchAcceptsCommittedText(t *testing.T) {
	app := trayQueryEditorTestApp(t)
	app.openFormTableEmojiPicker(0)
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return app.buildFormTableOverlay(snapshotFormTableEditorLocked(app.settingsTableEditor), uiPalette{}, 900, 700, 1)
	})
	host.AttachServices(formTableHostServices{})
	app.settingsHost = host
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 900, Height: 700}, PixelSize: woxui.PixelSize{Width: 900, Height: 700}, Scale: 1}
	displayList := woxui.DisplayList{}
	host.Frame(&displayList, frame)
	host.Frame(&displayList, frame)
	if !host.HasFocus("form-table-emoji-search") {
		t.Fatal("emoji search should receive initial focus")
	}
	if host.Key(woxui.KeyEvent{Key: "s", Down: true}) {
		t.Fatal("printable key should continue to native text input")
	}
	if !host.TextInput(woxui.TextInputEvent{Kind: woxui.TextInputCommit, Text: "符号"}) {
		t.Fatal("emoji search should accept committed text")
	}
	host.Frame(&displayList, frame)
	found := false
	for _, node := range host.Snapshot().Tree.Nodes {
		if node.AutomationID == "form-table-emoji-search" {
			found = true
			if node.Value != "符号" {
				t.Fatalf("emoji search value = %q, want 符号", node.Value)
			}
			break
		}
	}
	if !found {
		t.Fatal("emoji search semantics node was not published")
	}
}

// trayQueryEditorTestApp opens the tray query row editor for the first tray query.
func trayQueryEditorTestApp(t *testing.T) *App {
	t.Helper()
	deps := CommonDeps{}
	form := newHotkeySettingsForm(settingsData{
		MainHotkey:            "Alt+Space",
		SelectionHotkey:       "Alt+Shift+Space",
		TrayQueries:           json.RawMessage(`[{"Icon":{"ImageType":"emoji","ImageData":"📋"},"Query":"clipboard"}]`),
		IsLinuxWaylandSession: false,
	})
	hotkeys := newHotkeySettingsController(deps)
	hotkeys.SetForm(&form)
	app := &App{
		settingsOpen:   true,
		settingTab:     "general",
		hotkeySettings: hotkeys,
		aiSettings:     newAISettingsController(deps),
		pluginSettings: newPluginSettingsController(deps),
		settingsSearch: newSettingsSearchController(deps),
		themeSettings:  newThemeSettingsController(deps),
		sharedEdit:     newSharedEditState(),
		images:         map[string]*woxui.Image{},
		imageRequested: map[string]string{},
		imageLastUsed:  map[string]uint64{},
		imageErrors:    map[string]string{},
	}
	app.openTrayQueryEditor(0)
	if app.settingsTableEditor == nil || app.settingsTableEditor.rowForm == nil {
		t.Fatal("tray query row editor did not open")
	}
	return app
}
