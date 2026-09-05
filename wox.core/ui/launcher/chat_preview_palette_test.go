package launcher

import (
	"strings"
	"testing"
	"time"

	woxui "wox/ui/runtime"
)

func TestChatHistoryGroupUsesLocalDayBoundaries(t *testing.T) {
	location := time.FixedZone("test", 8*60*60)
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, location)
	if group := chatHistoryGroup(time.Date(2026, 8, 5, 0, 0, 0, 0, location).UnixMilli(), now); group != "today" {
		t.Fatalf("today group = %q", group)
	}
	if group := chatHistoryGroup(time.Date(2026, 8, 4, 23, 59, 0, 0, location).UnixMilli(), now); group != "yesterday" {
		t.Fatalf("yesterday group = %q", group)
	}
	if group := chatHistoryGroup(time.Date(2026, 8, 3, 23, 59, 0, 0, location).UnixMilli(), now); group != "history" {
		t.Fatalf("history group = %q", group)
	}
}

func TestSetChatTextKeepsHistorySidebarOpen(t *testing.T) {
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	ai.SetModels([]aiModel{{Name: "deepseek-v4-flash", Provider: "deepseek"}})
	ai.SetSkills(nil)
	app := &App{
		aiSettings: ai,
		chatPreview: &chatPreviewState{
			panel:  "history",
			editor: woxui.NewTextEditor(""),
			chat:   chatData{ID: "1", Title: "Suzhou"},
		},
	}

	app.setChatText("/")
	if !app.chatPreview.sidebarOpen {
		t.Fatal("typing / hid the conversation sidebar")
	}
	if app.chatPreview.panel != chatCommandPanel {
		t.Fatalf("panel = %q, want %q", app.chatPreview.panel, chatCommandPanel)
	}

	app.setChatText("hello")
	if app.chatPreview.panel != "history" || !app.chatPreview.sidebarOpen {
		t.Fatalf("clearing / did not restore the sidebar: panel=%q open=%v", app.chatPreview.panel, app.chatPreview.sidebarOpen)
	}
}

func TestDedicatedChatSnapshotAttachesModelCatalog(t *testing.T) {
	ai := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	ai.SetModels([]aiModel{{Name: "deepseek-v4-flash", Provider: "deepseek"}})
	app := &App{
		aiSettings: ai,
		chatPreview: &chatPreviewState{
			panel: chatCommandPanel,
			chat:  chatData{Model: aiModel{Name: "deepseek-v4-flash", Provider: "deepseek"}},
		},
	}

	snapshot := snapshotChatPreviewLocked(app.chatPreview)
	if len(snapshot.models) != 0 {
		t.Fatal("raw snapshot unexpectedly included models before attach")
	}
	app.attachChatPreviewCatalogs(snapshot)
	items := chatCommandPaletteItems(snapshot.models, snapshot.skills, snapshot.chat.Model, snapshot.panelQuery, snapshot.panel)
	if len(items) != 1 || items[0].title != "deepseek-v4-flash" {
		t.Fatalf("attached command palette = %+v, want the loaded model", items)
	}

	props := app.chatCatalogProps(snapshot, defaultPalette(), 400, 120)
	if len(props.Items) != 1 || props.Items[0].Title != "deepseek-v4-flash" {
		t.Fatalf("command catalog items = %+v empty=%q, want the loaded model", props.Items, props.EmptyMessage)
	}
}

func TestChatSkillTagRangesUsesRuneOffsets(t *testing.T) {
	text := "前 {skill:wox-plugin-creator} 后"
	ranges := chatSkillTagRanges(text)
	if len(ranges) != 1 || ranges[0].name != "wox-plugin-creator" {
		t.Fatalf("skill ranges = %+v", ranges)
	}
	runes := []rune(text)
	tag := "{skill:wox-plugin-creator}"
	if ranges[0].start != 2 || ranges[0].end != 2+len([]rune(tag)) || string(runes[ranges[0].start:ranges[0].end]) != tag {
		t.Fatalf("skill range = %+v, want rune span around the complete tag", ranges[0])
	}
	if got := chatSkillTagRanges("incomplete {skill:wox-plugin-creator"); len(got) != 0 {
		t.Fatalf("incomplete tag ranges = %+v, want none", got)
	}
}

func TestFindChatSlashTokenUsesTokenAtCaret(t *testing.T) {
	text := "hello /wri"
	token, ok := findChatSlashToken(woxui.TextEditingState{Text: text, Selection: woxui.TextSelection{Anchor: len([]rune(text)), Focus: len([]rune(text))}})
	if !ok || token.query != "wri" || token.start != 6 || token.end != 10 {
		t.Fatalf("slash token = %+v, %v", token, ok)
	}
}

func TestChatCommandPaletteFiltersModelsAndSkills(t *testing.T) {
	models := []aiModel{{Name: "deepseek-v4-pro", Provider: "deepseek"}}
	skills := []chatSkill{{Name: "writing-plans", Description: "Create an implementation plan", Source: "remote"}}

	items := chatCommandPaletteItems(models, skills, aiModel{}, "deep", chatCommandPanel)
	if len(items) != 1 || items[0].group != "models" || items[0].sourceIndex != 0 {
		t.Fatalf("model filter = %+v", items)
	}
	items = chatCommandPaletteItems(models, skills, aiModel{}, "implementation", chatCommandPanel)
	if len(items) != 1 || items[0].group != "skills" || items[0].sourceIndex != 0 {
		t.Fatalf("skill filter = %+v", items)
	}
}

func TestChatModelPaletteHeightShrinksToContentAndCaps(t *testing.T) {
	snapshot := &chatPreviewSnapshot{panel: "models", models: []aiModel{{Name: "flash"}, {Name: "pro"}}}
	if height := chatCatalogPanelHeight(snapshot, 600); height != 118 {
		t.Fatalf("two-model palette height = %.0f, want content height 118", height)
	}

	snapshot.models = make([]aiModel, 20)
	if height := chatCatalogPanelHeight(snapshot, 600); height != 310 {
		t.Fatalf("large model palette height = %.0f, want maximum 310", height)
	}
}

func TestReplaceChatSlashTokenWithSkillTag(t *testing.T) {
	editor := woxui.NewTextEditor("use /wri now")
	editor.SetCaret(8)
	replaceChatSlashToken(editor, "{skill:writing-plans}")
	state := editor.State()
	if state.Text != "use {skill:writing-plans} now" || state.Selection.Focus != 25 {
		t.Fatalf("replaced editor = %+v", state)
	}
}

func TestChatPaletteIgnoresKeyRelease(t *testing.T) {
	app := &App{chatPreview: &chatPreviewState{active: true, panel: chatCommandPanel, panelSelected: 2}}
	if app.onChatPreviewKey(woxui.KeyEvent{Key: woxui.KeyArrowDown}) {
		t.Fatal("key release was handled")
	}
	if app.chatPreview.panelSelected != 2 {
		t.Fatalf("selection moved to %d on key release", app.chatPreview.panelSelected)
	}
}

func TestPrimaryChatEscapeReturnsToQuery(t *testing.T) {
	app := &App{
		isPrimary:      true,
		chatFullscreen: true,
		chatPreview:    &chatPreviewState{active: true},
		editor:         woxui.NewTextEditor("chat "),
	}

	if !app.onChatPreviewKey(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) {
		t.Fatal("Escape was not handled")
	}
	if app.chatFullscreen || app.chatPreview == nil || app.chatPreview.active {
		t.Fatalf("chat mode state = fullscreen:%v preview:%+v", app.chatFullscreen, app.chatPreview)
	}
	if selection := app.editor.State().Selection; selection.Anchor != 0 || selection.Focus != len([]rune("chat ")) {
		t.Fatalf("query selection after escape = %+v, want full text selected", selection)
	}
}

func TestPrimaryChatShortcutTogglesHistorySidebar(t *testing.T) {
	app := &App{chatPreview: &chatPreviewState{active: true}}

	primaryModifier := woxui.KeyModifierControl
	if strings.HasPrefix(primaryHotkey("b"), "command+") {
		primaryModifier = woxui.KeyModifierMeta
	}
	event := woxui.KeyEvent{Key: woxui.Key("b"), Modifiers: primaryModifier, Down: true}
	if !app.onChatPreviewKey(event) {
		t.Fatal("primary+B was not handled")
	}
	if app.chatPreview.panel != "history" {
		t.Fatalf("history panel = %q after primary+B, want open", app.chatPreview.panel)
	}
	if !app.onChatPreviewKey(event) {
		t.Fatal("second primary+B was not handled")
	}
	if app.chatPreview.panel != "" {
		t.Fatalf("history panel = %q after second primary+B, want closed", app.chatPreview.panel)
	}
}

func TestHistoryDrawerLeavesComposerKeysToTheEditor(t *testing.T) {
	app := &App{chatPreview: &chatPreviewState{active: true, panel: "history", sidebarOpen: true, editor: woxui.NewTextEditor("")}}
	for _, event := range []woxui.KeyEvent{
		{Key: woxui.KeyEnter, Down: true, Modifiers: woxui.KeyModifierShift},
		{Key: woxui.KeyDelete, Down: true},
		{Key: woxui.KeyArrowUp, Down: true},
		{Key: woxui.KeyArrowDown, Down: true},
		{Key: woxui.KeyTab, Down: true},
	} {
		if app.onChatPreviewKey(event) {
			t.Fatalf("history drawer intercepted composer key %+v", event)
		}
	}
	// Empty-submit validation proves Enter reached send rather than history activation.
	if !app.onChatPreviewKey(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}) || app.chatPreview.error == "" {
		t.Fatal("Enter did not reach composer submission")
	}
	if !app.onChatPreviewKey(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) || app.chatPreview.panel != "" || !app.chatPreview.active {
		t.Fatal("Escape should dismiss the drawer while keeping the composer active")
	}
}

func TestChatHistoryContentHeightIncludesGroupsAndRows(t *testing.T) {
	location := time.FixedZone("test", 8*60*60)
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, location)
	chat := func(id string, updatedAt int64) chatData {
		return chatData{ID: id, UpdatedAt: updatedAt, Conversations: []chatConversation{{Role: "user", Text: "hi"}}}
	}
	chats := []chatData{
		chat("today-1", now.UnixMilli()),
		chat("today-2", now.UnixMilli()),
		chat("old", now.AddDate(0, 0, -5).UnixMilli()),
	}
	if got := chatHistoryContentHeight(chats, now); got != 46+2*32+3*38 {
		t.Fatalf("history content height = %.0f, want %d", got, 46+2*32+3*38)
	}
	if got := chatHistoryContentHeight(nil, now); got != 46 {
		t.Fatalf("empty history content height = %.0f, want 46", got)
	}
}

func TestChatHistoryWheelScrollUsesDrawerContentHeight(t *testing.T) {
	now := time.Now()
	chats := make([]chatData, 14)
	for i := range chats {
		chats[i] = chatData{ID: string(rune('a' + i)), UpdatedAt: now.UnixMilli(), Conversations: []chatConversation{{Role: "user", Text: "hi"}}}
	}
	// The old len*38 estimate kept maxOffset at 0 for moderate histories, so wheel scroll never moved.
	contentHeight := chatHistoryContentHeight(chats, time.Now())
	viewport := float32(576)
	app := &App{chatPreview: &chatPreviewState{panel: "history", chats: chats, panelViewport: viewport}}

	app.scrollChatPanel(120)
	want := min(float32(120), max(float32(0), contentHeight-viewport))
	if app.chatPreview.panelScroll != want {
		t.Fatalf("history panel scroll = %.0f, want %.0f (content %.0f, viewport %.0f)", app.chatPreview.panelScroll, want, contentHeight, viewport)
	}
	if app.chatPreview.panelScroll == 0 {
		t.Fatal("history wheel scroll stayed at 0 with overflow")
	}
}

func TestChatDebugGeometryClampsControlledScroll(t *testing.T) {
	app := &App{chatPreview: &chatPreviewState{panel: "debug", panelScroll: 500}}

	app.setChatDebugGeometry(100, 260)

	if app.chatPreview.panelMaxScroll != 160 || app.chatPreview.panelScroll != 160 {
		t.Fatalf("debug geometry = max %.0f offset %.0f, want 160/160", app.chatPreview.panelMaxScroll, app.chatPreview.panelScroll)
	}
}

func TestChatHistoryViewportUpdateKeepsWheelScroll(t *testing.T) {
	now := time.Now()
	chats := make([]chatData, 14)
	for i := range chats {
		chats[i] = chatData{ID: string(rune('a' + i)), UpdatedAt: now.UnixMilli(), Conversations: []chatConversation{{Role: "user", Text: "hi"}}}
	}
	app := &App{chatPreview: &chatPreviewState{panel: "history", chats: chats, panelViewport: 576}}

	app.scrollChatPanel(120)
	scrolled := app.chatPreview.panelScroll
	if scrolled == 0 {
		t.Fatal("wheel scroll did not move")
	}
	// The next build re-records the viewport and must not clamp the offset back to zero.
	app.setChatPanelViewport(576)
	if app.chatPreview.panelScroll != scrolled {
		t.Fatalf("viewport update changed scroll from %.0f to %.0f", scrolled, app.chatPreview.panelScroll)
	}
}
