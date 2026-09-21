package launcher

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"
	"wox/common/icons"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
)

func TestAboutMenuEntriesUseStableIDsAndLocalDispatch(t *testing.T) {
	entries := aboutMenuEntries("2.4.4")
	if len(entries) != 9 {
		t.Fatalf("about menu entries = %d, want 9 including the community header", len(entries))
	}
	want := []struct {
		id     string
		header bool
		url    string
		path   string
		query  string
	}{
		{id: aboutMenuFeedbackID, query: aboutMenuFeedbackQuery},
		{id: aboutMenuGuideID, url: aboutMenuGuideURL},
		{id: aboutMenuChangelogID, url: aboutMenuChangelogURL},
		{id: aboutMenuSettingsID, path: aboutMenuDefaultSettingsPath},
		{id: aboutMenuPluginStoreID, query: aboutMenuPluginStoreQuery},
		{id: aboutMenuCommunityID, header: true},
		{id: aboutMenuGithubID, url: aboutMenuGithubURL},
		{id: aboutMenuRedditID, url: aboutMenuRedditURL},
		{id: aboutMenuDiscordID, url: aboutMenuDiscordURL},
	}
	for index, entry := range entries {
		item := want[index]
		if entry.ID != item.id || entry.IsGroupHeader != item.header || entry.Source != actionPanelSourceLocal {
			t.Fatalf("entry %d = id %q header %v source %v", index, entry.ID, entry.IsGroupHeader, entry.Source)
		}
		if url, ok := aboutMenuExternalURL(entry.ID, "en_US"); ok != (item.url != "") || url != item.url {
			t.Fatalf("url for %s = %q ok %v, want %q", entry.ID, url, ok, item.url)
		}
		if path, ok := aboutMenuSettingsRoute(entry.ID); ok != (item.path != "") || path != item.path {
			t.Fatalf("settings path for %s = %q ok %v, want %q", entry.ID, path, ok, item.path)
		}
		if query, ok := aboutMenuQuery(entry.ID); ok != (item.query != "") || query != item.query {
			t.Fatalf("query for %s = %q ok %v, want %q", entry.ID, query, ok, item.query)
		}
	}
	if aboutMenuFeedbackQuery != "feedback " {
		t.Fatalf("feedback query = %q, want %q", aboutMenuFeedbackQuery, "feedback ")
	}
	if aboutMenuPluginStoreQuery != "wpm install " {
		t.Fatalf("plugin store query = %q, want %q", aboutMenuPluginStoreQuery, "wpm install ")
	}
	if aboutMenuGuideURL != "https://www.woxlauncher.com/guide/introduction.html" {
		t.Fatalf("guide url = %q", aboutMenuGuideURL)
	}
}

func TestAboutMenuGuideURLFollowsChineseLanguage(t *testing.T) {
	got, ok := aboutMenuExternalURL(aboutMenuGuideID, "zh_CN")
	if !ok || got != aboutMenuGuideURLZh {
		t.Fatalf("zh_CN guide = %q ok %v, want %q", got, ok, aboutMenuGuideURLZh)
	}
	got, ok = aboutMenuExternalURL(aboutMenuGuideID, "en_US")
	if !ok || got != aboutMenuGuideURL {
		t.Fatalf("en_US guide = %q ok %v, want %q", got, ok, aboutMenuGuideURL)
	}
	got, ok = aboutMenuExternalURL(aboutMenuGuideID, "ja_JP")
	if !ok || got != aboutMenuGuideURL {
		t.Fatalf("ja_JP guide = %q ok %v, want %q", got, ok, aboutMenuGuideURL)
	}
}

func TestActionPanelHeaderLabelUsesAboutTitle(t *testing.T) {
	if got := actionPanelHeaderLabel(viewSnapshot{actionPanelPurpose: actionPanelPurposeAbout}, "Actions", "About"); got != "About" {
		t.Fatalf("about header = %q", got)
	}
	if got := actionPanelHeaderLabel(viewSnapshot{}, "Actions", "About"); got != "Actions" {
		t.Fatalf("result header = %q", got)
	}
}

func TestAboutMenuVersionTail(t *testing.T) {
	if got := aboutMenuVersionTail(""); got != "" {
		t.Fatalf("empty version = %q", got)
	}
	if got := aboutMenuVersionTail(" 2.4.4 "); got != "v2.4.4" {
		t.Fatalf("version tail = %q", got)
	}
	if got := aboutMenuVersionTail("v2.4.4"); got != "v2.4.4" {
		t.Fatalf("prefixed version tail = %q", got)
	}
}

func TestAboutMenuButtonVisibleDefersMessageWhileOpen(t *testing.T) {
	message := &toolbarMessage{Title: "Indexing"}
	if !aboutMenuButtonVisible(false, actionPanelPurposeResult, nil) {
		t.Fatal("closed panel with no message should show the about menu button")
	}
	if aboutMenuButtonVisible(false, actionPanelPurposeResult, message) {
		t.Fatal("closed panel with a message should hide the about menu button")
	}
	if !aboutMenuButtonVisible(true, actionPanelPurposeAbout, message) {
		t.Fatal("open about menu should keep the button even when a message arrives")
	}
}

func TestAboutMenuHotkeyOpensAndClosesPanel(t *testing.T) {
	modifiers := woxui.KeyModifierControl | woxui.KeyModifierShift
	if runtime.GOOS == "darwin" {
		modifiers = woxui.KeyModifierMeta | woxui.KeyModifierShift
	}
	app := &App{generalSettings: &generalSettingsController{}}
	event := woxui.KeyEvent{Key: "k", Modifiers: modifiers, Down: true}
	if !app.onActionKey(event) || !app.actionPanel || app.actionPanelPurpose != actionPanelPurposeAbout {
		t.Fatal("about menu hotkey should open the left panel")
	}
	if !app.onActionKey(event) || app.actionPanel {
		t.Fatal("about menu hotkey should close an already open about menu")
	}
}

func TestAboutMenuHotkeyMatchesPrimaryShiftK(t *testing.T) {
	modifiers := woxui.KeyModifierControl | woxui.KeyModifierShift
	if runtime.GOOS == "darwin" {
		modifiers = woxui.KeyModifierMeta | woxui.KeyModifierShift
	}
	if !hotkeyMatches(aboutMenuHotkey(), woxui.KeyEvent{Key: "k", Modifiers: modifiers, Down: true}) {
		t.Fatalf("about menu hotkey %q should match primary+shift+k", aboutMenuHotkey())
	}
	if hotkeyMatches(aboutMenuHotkey(), woxui.KeyEvent{Key: "k", Modifiers: modifiers, Down: true, Composing: true}) {
		t.Fatal("IME composition must not toggle the about menu")
	}
	app := &App{generalSettings: &generalSettingsController{}}
	if !app.onActionKey(woxui.KeyEvent{Key: "k", Modifiers: modifiers, Down: true, Repeat: true}) {
		t.Fatal("repeat about-menu hotkey should be consumed")
	}
	if app.actionPanel {
		t.Fatal("repeat about-menu hotkey must not toggle the panel")
	}
}

func TestToggleAboutMenuAndResultPanelAreExclusive(t *testing.T) {
	app := &App{generalSettings: &generalSettingsController{}, editor: woxui.NewTextEditor(""), results: []queryResult{{Actions: []resultAction{{ID: "open", Name: "Open"}}}}}
	app.toggleAboutMenu()
	if !app.actionPanel || app.actionPanelPurpose != actionPanelPurposeAbout {
		t.Fatal("about menu should open")
	}
	if app.actionFilter == nil || app.actionFilter.State().Text != "" {
		t.Fatal("opening about menu should clear the filter")
	}
	app.actionFilter.SetText("guide", false)
	app.toggleActionPanel()
	if !app.actionPanel || app.actionPanelPurpose != actionPanelPurposeResult {
		t.Fatal("result panel should replace the about menu")
	}
	if app.actionFilter.State().Text != "" {
		t.Fatal("switching panels should clear retained filter state")
	}
	app.toggleAboutMenu()
	if app.actionPanelPurpose != actionPanelPurposeAbout {
		t.Fatal("about menu should replace the result panel")
	}
}

func TestToggleActionPanelDoesNotCloseAboutMenuWithoutResultActions(t *testing.T) {
	app := &App{generalSettings: &generalSettingsController{}, editor: woxui.NewTextEditor("")}
	app.toggleAboutMenu()
	app.toggleActionPanel()
	if !app.actionPanel || app.actionPanelPurpose != actionPanelPurposeAbout {
		t.Fatal("a result-panel request without actions must not destroy the about menu")
	}
}

func TestToolbarAndResultUpdatesKeepAboutMenu(t *testing.T) {
	app := &App{generalSettings: &generalSettingsController{}, editor: woxui.NewTextEditor(""), actionFilter: woxui.NewTextEditor("guide")}
	app.toggleAboutMenu()
	app.actionSelectionKey = "about:guide"
	app.actionSelected = 1
	if app.shouldSyncActionPanelWithResults() {
		t.Fatal("about menu must not resync from result or toolbar updates")
	}
	app.actionFilter.SetText("guide", false)
	app.query = plainQuery{QueryID: "about-query", QueryText: "example"}
	app.applyToolbarFallbackMessage(toolbarMessage{ID: "fallback", Title: "Warning"})
	app.applyToolbarMessage(toolbarMessage{Title: "Indexing", ID: "msg-a"})
	if !app.actionPanel || app.actionPanelPurpose != actionPanelPurposeAbout || app.actionSelectionKey != "about:guide" {
		t.Fatal("storing a toolbar message must not change the open about menu")
	}
	app.applyToolbarMessage(toolbarMessage{Title: "Done", ID: "msg-b"})
	app.applyResults("about-query", nil, &queryLayout{}, nil, nil, 0, true)
	if app.actionFilter.State().Text != "guide" || app.actionSelectionKey != "about:guide" {
		t.Fatal("updates changed menu selection or filter")
	}
	app.clearToolbarMessageByID("msg-b")
	if app.effectiveToolbarMessage().ID != "fallback" || !app.actionPanel {
		t.Fatal("clear must restore fallback without closing menu")
	}
	app.applyToolbarMessage(toolbarMessage{Title: "Done", ID: "msg-b"})
	if !aboutMenuButtonVisible(app.actionPanel, app.actionPanelPurpose, app.effectiveToolbarMessage()) {
		t.Fatal("open about menu should keep the toolbar button while a newer message is stored")
	}
	app.hideActionPanel()
	if app.effectiveToolbarMessage() == nil || app.effectiveToolbarMessage().ID != "msg-b" {
		t.Fatal("closing the about menu should present the latest stored message")
	}
}

func TestAboutMenuCurrentEntriesIgnoreResultRefresh(t *testing.T) {
	app := &App{actionPanel: true, actionPanelPurpose: actionPanelPurposeAbout, results: []queryResult{{Actions: []resultAction{{ID: "open", Name: "Open"}}}}}
	entries := app.currentActionPanelEntries()
	if len(entries) == 0 || entries[0].ID != aboutMenuFeedbackID {
		t.Fatalf("about panel entries = %#v", entries)
	}
	if got := app.resultActionPanelEntries(); len(got) == 0 || got[0].ID != "result-open-0" {
		t.Fatalf("result entries = %#v", got)
	}
}

func TestAboutMenuDisplayItemsInsertCommunityHeaderAfterFilter(t *testing.T) {
	entries := aboutMenuEntries("2.4.4")
	items := actionPanelDisplayItems(entries, actionPanelUnfilteredIndices(entries), nil)
	if len(items) != 9 || items[5].Kind != launcherview.ActionItemKindGroupHeader || items[5].ID != aboutMenuCommunityID {
		t.Fatalf("unfiltered items = %#v, want community header before github", items)
	}
	filtered := filteredActionIndices(entries, "red", nil, false)
	filteredItems := actionPanelDisplayItems(entries, filtered, nil)
	if len(filteredItems) != 2 || filteredItems[0].Kind != launcherview.ActionItemKindGroupHeader || filteredItems[1].ID != aboutMenuRedditID {
		t.Fatalf("reddit filter items = %#v", filteredItems)
	}
	feedback := filteredActionIndices(entries, "feedback", map[string]string{"ui_about_menu_feedback": "Send Feedback"}, false)
	feedbackItems := actionPanelDisplayItems(entries, feedback, nil)
	if len(feedbackItems) != 1 || feedbackItems[0].ID != aboutMenuFeedbackID {
		t.Fatalf("feedback filter should omit the community header: %#v", feedbackItems)
	}
}

func TestAboutMenuErrorIsVisibleWithoutReplacingToolbarMessage(t *testing.T) {
	app := &App{generalSettings: &generalSettingsController{}, toolbarMsg: &toolbarMessage{ID: "progress", Title: "Indexing"}}
	app.toggleAboutMenu()
	app.notifyAboutMenuError("Could not open the link")
	snapshot := viewSnapshot{actionPanelPurpose: app.actionPanelPurpose, aboutMenuError: app.aboutMenuError}
	if got := actionPanelHeaderLabel(snapshot, "Actions", "2.0"); got != "Could not open the link" {
		t.Fatalf("visible header = %q", got)
	}
	if !app.actionPanel || app.toolbarMsg.ID != "progress" {
		t.Fatal("error must preserve menu and background message")
	}
	app.hideActionPanel()
	if app.aboutMenuError != "" {
		t.Fatal("closing must clear the error")
	}
}

func TestAboutMenuMessageExpiresThroughUIWhileOpen(t *testing.T) {
	app := &App{lifecycleCtx: context.Background(), generalSettings: &generalSettingsController{}, editor: woxui.NewTextEditor("")}
	app.toggleAboutMenu()
	dispatched := make(chan func(), 2)
	app.uiCall = func(fn func()) error { dispatched <- fn; return nil }
	app.applyToolbarMessage(toolbarMessage{Text: "Temporary", DisplaySeconds: 1})
	// Apply the synchronous bounds callback on the test's UI goroutine.
	(<-dispatched)()
	select {
	case fn := <-dispatched:
		fn()
	case <-time.After(3 * time.Second):
		t.Fatal("message expiry was not dispatched")
	}
	if app.toolbarMsg != nil || !app.actionPanel || app.actionPanelPurpose != actionPanelPurposeAbout {
		t.Fatal("expiry must clear only the message")
	}
}

func TestActionPanelFilterKeepsNamedGroupsTogether(t *testing.T) {
	entries := []actionPanelEntry{
		{ID: "guide", Name: "Guide"},
		{ID: "community", Name: "Community", IsGroupHeader: true},
		{ID: "discord", Name: "Discord"},
		{ID: "reddit", Name: "Reddit"},
	}
	indices := filteredActionIndices(entries, "d", nil, false)
	if len(indices) != 3 || indices[0] != 0 {
		t.Fatalf("group order = %v", indices)
	}
	items := actionPanelDisplayItems(entries, indices, nil)
	if len(items) != 4 || items[0].ID != "guide" || items[1].Kind != launcherview.ActionItemKindGroupHeader {
		t.Fatalf("items = %#v", items)
	}
}

func TestAboutMenuTails(t *testing.T) {
	byID := map[string]actionPanelEntry{}
	for _, entry := range aboutMenuEntries("2.4.4") {
		byID[entry.ID] = entry
		if entry.Hotkey != "" {
			t.Fatalf("%s must not show an Enter hotkey", entry.ID)
		}
	}
	if byID[aboutMenuFeedbackID].TailIcon != fromCoreImage(icons.Get(icons.PluginFeedback)) {
		t.Fatal("feedback tail must be the Feedback plugin icon")
	}
	if byID[aboutMenuPluginStoreID].TailIcon != fromCoreImage(icons.Get(icons.PluginWPM)) {
		t.Fatal("plugin store tail must be the WPM plugin icon")
	}
	if byID[aboutMenuChangelogID].Tail != "v2.4.4" {
		t.Fatalf("changelog tail = %q", byID[aboutMenuChangelogID].Tail)
	}
	if byID[aboutMenuSettingsID].TailIcon != fromCoreImage(icons.Get(icons.BrandWox)) {
		t.Fatal("settings tail must be the Wox logo")
	}
	if byID[aboutMenuGuideID].Tail != aboutMenuGuideTail {
		t.Fatalf("guide tail = %q", byID[aboutMenuGuideID].Tail)
	}
	if byID[aboutMenuGithubID].Tail != aboutMenuGithubTail {
		t.Fatalf("github tail = %q", byID[aboutMenuGithubID].Tail)
	}
	if byID[aboutMenuRedditID].Tail != aboutMenuRedditTail {
		t.Fatalf("reddit tail = %q", byID[aboutMenuRedditID].Tail)
	}
	if byID[aboutMenuDiscordID].Tail != "i18n:ui_about_menu_discord_tail" {
		t.Fatalf("discord tail = %q", byID[aboutMenuDiscordID].Tail)
	}
	if got := aboutMenuDiscordTail("%d+ users"); got != fmt.Sprintf("%d+ users", aboutMenuDiscordUserCount) {
		t.Fatalf("discord member tail = %q", got)
	}
	for _, id := range []string{aboutMenuCommunityID} {
		if byID[id].Tail != "" || byID[id].TailIcon.ImageData != "" {
			t.Fatalf("%s should have no tail", id)
		}
	}
}
