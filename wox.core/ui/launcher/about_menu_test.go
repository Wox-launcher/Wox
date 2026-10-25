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
	woxwidget "wox/ui/widget"
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
	app := bindTestApp(&App{generalSettings: &generalSettingsController{}})
	event := woxui.KeyEvent{Key: "k", Modifiers: modifiers, Down: true}
	if !app.actions.onActionKey(event) || !app.actions.actionPanel || app.actions.actionPanelPurpose != actionPanelPurposeAbout {
		t.Fatal("about menu hotkey should open the left panel")
	}
	if !app.actions.onActionKey(event) || app.actions.actionPanel {
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
	app := bindTestApp(&App{generalSettings: &generalSettingsController{}})
	if !app.actions.onActionKey(woxui.KeyEvent{Key: "k", Modifiers: modifiers, Down: true, Repeat: true}) {
		t.Fatal("repeat about-menu hotkey should be consumed")
	}
	if app.actions.actionPanel {
		t.Fatal("repeat about-menu hotkey must not toggle the panel")
	}
}

func TestToggleAboutMenuAndResultPanelAreExclusive(t *testing.T) {
	app := bindTestApp(&App{
		generalSettings: &generalSettingsController{},
		querySession:    queryController{editor: woxui.NewTextEditor("")},
		resultList:      resultController{results: []queryResult{{Actions: []resultAction{{ID: "open", Name: "Open"}}}}},
	})
	app.toggleAboutMenu()
	if !app.actions.actionPanel || app.actions.actionPanelPurpose != actionPanelPurposeAbout {
		t.Fatal("about menu should open")
	}
	if app.actions.actionFilter == nil || app.actions.actionFilter.State().Text != "" {
		t.Fatal("opening about menu should clear the filter")
	}
	app.actions.actionFilter.SetText("guide", false)
	app.actions.toggleActionPanel()
	if !app.actions.actionPanel || app.actions.actionPanelPurpose != actionPanelPurposeResult {
		t.Fatal("result panel should replace the about menu")
	}
	if app.actions.actionFilter.State().Text != "" {
		t.Fatal("switching panels should clear retained filter state")
	}
	app.toggleAboutMenu()
	if app.actions.actionPanelPurpose != actionPanelPurposeAbout {
		t.Fatal("about menu should replace the result panel")
	}
}

func TestToggleActionPanelDoesNotCloseAboutMenuWithoutResultActions(t *testing.T) {
	app := bindTestApp(&App{
		generalSettings: &generalSettingsController{},
		querySession:    queryController{editor: woxui.NewTextEditor("")},
	})
	app.toggleAboutMenu()
	app.actions.toggleActionPanel()
	if !app.actions.actionPanel || app.actions.actionPanelPurpose != actionPanelPurposeAbout {
		t.Fatal("a result-panel request without actions must not destroy the about menu")
	}
}

func TestToolbarAndResultUpdatesKeepAboutMenu(t *testing.T) {
	app := bindTestApp(&App{
		generalSettings: &generalSettingsController{},
		querySession:    queryController{editor: woxui.NewTextEditor("")},
		actions:         actionPanelController{actionFilter: woxui.NewTextEditor("guide")},
	})
	app.toggleAboutMenu()
	app.actions.actionSelectionKey = "about:guide"
	app.actions.actionSelected = 1
	if app.actions.shouldSyncActionPanelWithResults() {
		t.Fatal("about menu must not resync from result or toolbar updates")
	}
	app.actions.actionFilter.SetText("guide", false)
	app.querySession.query = plainQuery{QueryID: "about-query", QueryText: "example"}
	app.toolbar.applyToolbarFallbackMessage(toolbarMessage{ID: "fallback", Title: "Warning"})
	app.toolbar.applyToolbarMessage(toolbarMessage{Title: "Indexing", ID: "msg-a"})
	if !app.actions.actionPanel || app.actions.actionPanelPurpose != actionPanelPurposeAbout || app.actions.actionSelectionKey != "about:guide" {
		t.Fatal("storing a toolbar message must not change the open about menu")
	}
	app.toolbar.applyToolbarMessage(toolbarMessage{Title: "Done", ID: "msg-b"})
	app.applyResults("about-query", nil, &queryLayout{}, nil, nil, 0, true)
	if app.actions.actionFilter.State().Text != "guide" || app.actions.actionSelectionKey != "about:guide" {
		t.Fatal("updates changed menu selection or filter")
	}
	app.toolbar.clearToolbarMessageByID("msg-b")
	if app.toolbar.effectiveToolbarMessage().ID != "fallback" || !app.actions.actionPanel {
		t.Fatal("clear must restore fallback without closing menu")
	}
	app.toolbar.applyToolbarMessage(toolbarMessage{Title: "Done", ID: "msg-b"})
	if !aboutMenuButtonVisible(app.actions.actionPanel, app.actions.actionPanelPurpose, app.toolbar.effectiveToolbarMessage()) {
		t.Fatal("open about menu should keep the toolbar button while a newer message is stored")
	}
	app.actions.hideActionPanel()
	if app.toolbar.effectiveToolbarMessage() == nil || app.toolbar.effectiveToolbarMessage().ID != "msg-b" {
		t.Fatal("closing the about menu should present the latest stored message")
	}
}

func TestAboutMenuCurrentEntriesIgnoreResultRefresh(t *testing.T) {
	app := bindTestApp(&App{
		actions: actionPanelController{
			actionPanel:        true,
			actionPanelPurpose: actionPanelPurposeAbout,
		},
		resultList: resultController{results: []queryResult{{Actions: []resultAction{{ID: "open", Name: "Open"}}}}},
	})
	entries := app.actions.currentActionPanelEntries()
	if len(entries) == 0 || entries[0].ID != aboutMenuFeedbackID {
		t.Fatalf("about panel entries = %#v", entries)
	}
	if got := app.resultActionPanelEntries(); len(got) == 0 || got[0].ID != "result-open-0" {
		t.Fatalf("result entries = %#v", got)
	}
}

func TestAboutMenuDisplayItemsInsertCommunityHeaderAfterFilter(t *testing.T) {
	entries := aboutMenuEntries("2.4.4")
	items := actionPanelDisplayItems(entries, actionPanelUnfilteredIndices(entries), false, nil)
	if len(items) != 9 || items[5].Kind != launcherview.ActionItemKindGroupHeader || items[5].ID != aboutMenuCommunityID {
		t.Fatalf("unfiltered items = %#v, want community header before github", items)
	}
	filtered := filteredActionIndices(entries, "red", nil, false)
	filteredItems := actionPanelDisplayItems(entries, filtered, false, nil)
	if len(filteredItems) != 2 || filteredItems[0].Kind != launcherview.ActionItemKindGroupHeader || filteredItems[1].ID != aboutMenuRedditID {
		t.Fatalf("reddit filter items = %#v", filteredItems)
	}
	feedback := filteredActionIndices(entries, "feedback", map[string]string{"ui_about_menu_feedback": "Send Feedback"}, false)
	feedbackItems := actionPanelDisplayItems(entries, feedback, false, nil)
	if len(feedbackItems) != 1 || feedbackItems[0].ID != aboutMenuFeedbackID {
		t.Fatalf("feedback filter should omit the community header: %#v", feedbackItems)
	}
}

func TestAboutMenuErrorIsVisibleWithoutReplacingToolbarMessage(t *testing.T) {
	app := bindTestApp(&App{
		generalSettings: &generalSettingsController{},
		toolbar:         toolbarController{toolbarMsg: &toolbarMessage{ID: "progress", Title: "Indexing"}},
	})
	app.toggleAboutMenu()
	app.notifyAboutMenuError("Could not open the link")
	snapshot := viewSnapshot{actionPanelPurpose: app.actions.actionPanelPurpose, aboutMenuError: app.actions.aboutMenuError}
	if got := actionPanelHeaderLabel(snapshot, "Actions", "2.0"); got != "Could not open the link" {
		t.Fatalf("visible header = %q", got)
	}
	if !app.actions.actionPanel || app.toolbar.toolbarMsg.ID != "progress" {
		t.Fatal("error must preserve menu and background message")
	}
	app.actions.hideActionPanel()
	if app.actions.aboutMenuError != "" {
		t.Fatal("closing must clear the error")
	}
}

func TestAboutMenuMessageExpiresThroughUIWhileOpen(t *testing.T) {
	app := bindTestApp(&App{
		lifecycleCtx:    context.Background(),
		generalSettings: &generalSettingsController{},
		querySession:    queryController{editor: woxui.NewTextEditor("")},
	})
	app.toggleAboutMenu()
	dispatched := make(chan func(), 2)
	app.uiCall = func(fn func()) error { dispatched <- fn; return nil }
	app.toolbar.applyToolbarMessage(toolbarMessage{Text: "Temporary", DisplaySeconds: 1})
	// Apply the synchronous bounds callback on the test's UI goroutine.
	(<-dispatched)()
	select {
	case fn := <-dispatched:
		fn()
	case <-time.After(3 * time.Second):
		t.Fatal("message expiry was not dispatched")
	}
	if app.toolbar.toolbarMsg != nil || !app.actions.actionPanel || app.actions.actionPanelPurpose != actionPanelPurposeAbout {
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
	items := actionPanelDisplayItems(entries, indices, false, nil)
	if len(items) != 4 || items[0].ID != "guide" || items[1].Kind != launcherview.ActionItemKindGroupHeader {
		t.Fatalf("items = %#v", items)
	}
}

func TestAboutMenuTails(t *testing.T) {
	byID := map[string]actionPanelEntry{}
	for _, entry := range aboutMenuEntries("2.4.4") {
		byID[entry.ID] = entry
		if entry.ID != aboutMenuSettingsID && entry.Hotkey != "" {
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
	if byID[aboutMenuSettingsID].TailIcon.ImageData != "" {
		t.Fatal("settings shortcut must not be accompanied by a tail icon")
	}
	if byID[aboutMenuSettingsID].Hotkey != primaryHotkey(",") {
		t.Fatalf("settings shortcut = %q", byID[aboutMenuSettingsID].Hotkey)
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

// TestAboutMenuSettingsShortcutIsRendered guards against suppressing all About-menu keycaps.
func TestAboutMenuSettingsShortcutIsRendered(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	entries := aboutMenuEntries("2.4.6")
	// Icon identities are covered separately; this headless test only exercises keycap rendering.
	for index := range entries {
		entries[index].Icon = woxImage{}
		entries[index].TailIcon = woxImage{}
	}
	panel, _, _ := app.buildActionPanel(viewSnapshot{
		palette: defaultPalette(), actionPanelPurpose: actionPanelPurposeAbout,
		actionEntries: entries, actionIndices: actionPanelUnfilteredIndices(entries),
	}, 600, 600, 60, 40, 1)
	props := panel.(woxwidget.Stateful).Widget.(launcherview.ActionsProps)
	modifier := "Ctrl"
	if runtime.GOOS == "darwin" {
		modifier = "Cmd"
	}
	found := false
	for _, item := range props.Items {
		if item.ID == aboutMenuSettingsID {
			found = true
			if len(item.HotkeyLabels) != 2 || item.HotkeyLabels[0] != modifier || item.HotkeyLabels[1] != "," {
				t.Fatalf("settings row lost its shortcut: %+v", item)
			}
		} else if len(item.HotkeyLabels) != 0 {
			t.Fatalf("About row %q gained an unrelated shortcut", item.ID)
		}
	}
	if !found {
		t.Fatal("settings row was not rendered")
	}
}
