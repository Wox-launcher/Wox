package launcher

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"wox/common"
	"wox/plugin"
	"wox/ui/contract"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	utilselection "wox/util/selection"
)

type requestMRUTestServices struct {
	contract.Services
}

func (requestMRUTestServices) QueryMRU(context.Context, string, string) ([]plugin.QueryResultUI, error) {
	return nil, errors.New("stop after request")
}

type mruResultsTestServices struct {
	contract.Services
}

func (mruResultsTestServices) QueryMRU(context.Context, string, string) ([]plugin.QueryResultUI, error) {
	return []plugin.QueryResultUI{{Id: "mru"}}, nil
}

type remotePreviewTestServices struct {
	contract.Services
	called chan struct{}
}

func (s *remotePreviewTestServices) ResultPreview(context.Context, string, string, string, string) (plugin.WoxPreview, error) {
	close(s.called)
	return plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeText, PreviewData: "loaded"}, nil
}

func TestLauncherWindowOriginPreservesDraggedPosition(t *testing.T) {
	params := showAppParams{Position: position{X: 400, Y: 300}}
	current := woxui.Rect{X: 92, Y: 74, Width: 760, Height: 420}

	x, y := launcherWindowOrigin(params, current, 620, false)
	if x != current.X || y != current.Y {
		t.Fatalf("preserved origin = %.0f,%.0f, want %.0f,%.0f", x, y, current.X, current.Y)
	}
}

func TestLauncherPreviewRatioUsesChatLayout(t *testing.T) {
	ratio := 0.25
	if got := launcherPreviewRatio(queryLayout{ResultPreviewWidthRatio: &ratio}, false); got != 0.25 {
		t.Fatalf("regular preview ratio = %v, want 0.25", got)
	}
	if got := launcherPreviewRatio(queryLayout{ResultPreviewWidthRatio: &ratio, ChatMode: true}, false); got != 0 {
		t.Fatalf("chat preview ratio = %v, want 0", got)
	}
	if got := launcherPreviewRatio(queryLayout{ResultPreviewWidthRatio: &ratio}, true); got != 0 {
		t.Fatalf("fullscreen preview ratio = %v, want 0", got)
	}
}

func TestLauncherToolbarHeightIncludedInChatFullscreen(t *testing.T) {
	if !launcherToolbarHeightIncluded(false, true, true, true) {
		t.Fatal("chat fullscreen should retain the hidden toolbar height")
	}
	if launcherToolbarHeightIncluded(false, true, true, false) {
		t.Fatal("terminal fullscreen should not retain the hidden toolbar height")
	}
	if launcherToolbarHeightIncluded(true, true, true, true) || launcherToolbarHeightIncluded(false, false, true, true) {
		t.Fatal("disabled or empty toolbar should not contribute height")
	}
}

func TestApplyResultsEntersChatModeFromLayout(t *testing.T) {
	app := New(false, nil)
	app.uiCall = nil
	app.visible = true
	app.query = newInputQuery("chat ")
	defer app.cancel()

	app.applyResults(app.query.QueryID, []queryResult{{
		ID: "chat",
		Preview: queryPreview{
			PreviewType: "chat",
			PreviewData: `{"ActiveChat":{"Id":"chat"}}`,
		},
	}}, &queryLayout{ChatMode: true}, nil, nil, 0, true)

	if !app.chatFullscreen || app.chatPreview == nil || !app.chatPreview.active {
		t.Fatalf("chat mode state = fullscreen:%v preview:%+v", app.chatFullscreen, app.chatPreview)
	}
}

func TestLauncherGridHidesRegularPreview(t *testing.T) {
	layout := queryLayout{GridLayout: &gridLayout{Columns: 4}}
	if launcherPreviewVisible(layout, queryPreview{PreviewType: "image", PreviewData: "wallpaper"}) {
		t.Fatal("grid layout should hide regular result previews")
	}
	if !launcherPreviewVisible(layout, queryPreview{PreviewType: "query_requirement_settings", PreviewData: "settings"}) {
		t.Fatal("grid layout should preserve Flutter's interactive settings preview exception")
	}
}

func TestMRUResultsResetGridLayout(t *testing.T) {
	app := New(false, mruResultsTestServices{})
	app.uiCall = nil
	app.layout = queryLayout{GridLayout: &gridLayout{Columns: 4}}
	defer app.cancel()

	app.loadTypedMRU(app.query.QueryID)

	if app.layout.GridLayout != nil {
		t.Fatal("MRU results retained the previous query's grid layout")
	}
}

func TestLauncherChromeHiddenForPreviewOnlyModes(t *testing.T) {
	if !launcherChromeHidden(showAppParams{HideQueryBox: true, HideToolbar: true}, false) {
		t.Fatal("hidden query box and toolbar should expose preview close behavior")
	}
	if !launcherChromeHidden(showAppParams{}, true) {
		t.Fatal("chat fullscreen should expose preview close behavior")
	}
	if launcherChromeHidden(showAppParams{HideQueryBox: true}, false) {
		t.Fatal("partially hidden launcher chrome should keep normal navigation behavior")
	}
}

func TestLauncherPreviewOnlyRequiresChromeFreeZeroRatioPreview(t *testing.T) {
	ratio := 0.0
	snapshot := viewSnapshot{
		selected: 0,
		results:  []queryResult{{Preview: queryPreview{PreviewType: "file", PreviewData: "report.xlsx"}}},
		layout:   queryLayout{ResultPreviewWidthRatio: &ratio},
		show:     showAppParams{HideQueryBox: true, HideToolbar: true},
	}
	if !launcherPreviewOnly(snapshot) {
		t.Fatal("chrome-free zero-ratio preview should enable border dragging")
	}

	snapshot.show.HideToolbar = false
	if launcherPreviewOnly(snapshot) {
		t.Fatal("visible launcher chrome should not enable preview-only border dragging")
	}
	snapshot.show.HideToolbar = true
	ratio = 0.4
	if launcherPreviewOnly(snapshot) {
		t.Fatal("split preview layout should not enable preview-only border dragging")
	}
}

func TestLauncherPreviewTitleBarRequiresOptInFullPreview(t *testing.T) {
	ratio := 0.0
	snapshot := viewSnapshot{
		selected: 0,
		results:  []queryResult{{Title: "Document", Preview: queryPreview{PreviewType: "file", PreviewData: "report.docx"}}},
		layout:   queryLayout{ResultPreviewWidthRatio: &ratio},
		show:     showAppParams{HideQueryBox: true, HideToolbar: true, ShowPreviewTitleBar: true},
	}
	if !launcherPreviewTitleBarVisible(snapshot) {
		t.Fatal("opt-in full preview should show the title bar")
	}

	snapshot.show.ShowPreviewTitleBar = false
	if launcherPreviewTitleBarVisible(snapshot) {
		t.Fatal("title bar should remain hidden by default")
	}
	snapshot.show.ShowPreviewTitleBar = true
	snapshot.results[0].Preview.PreviewType = "chat"
	if launcherPreviewTitleBarVisible(snapshot) {
		t.Fatal("chat preview should keep its existing header")
	}
	snapshot.results[0].Preview.PreviewType = "file"
	ratio = 0.4
	if launcherPreviewTitleBarVisible(snapshot) {
		t.Fatal("split preview should not show the full-preview title bar")
	}
}

func TestSecondaryLauncherHideClosesWithoutWebViewCache(t *testing.T) {
	app := &App{isPrimary: false}
	app.destroyOnce.Do(func() {})
	if err := app.hideWindow(false); err != nil {
		t.Fatalf("non-WebView secondary hide should close the instance: %v", err)
	}
}

func TestSecondaryLauncherHideRetainsCacheableWebView(t *testing.T) {
	// WebView secondaries are independent named windows; only they hide-and-retain
	// so browsing position survives reopen. Selection/explorer/tray still destroy.
	app := &App{
		isPrimary:          false,
		visible:            false,
		webViewPreviewData: `{"url":"https://example.com","cacheDisabled":false}`,
	}
	if err := app.hideWindow(false); err != nil {
		t.Fatalf("cacheable WebView secondary hide should preserve the instance: %v", err)
	}
	if app.destroyed.Load() {
		t.Fatal("cacheable WebView secondary must stay alive so navigation can resume")
	}
}

func TestHasCacheableWebViewPreviewIgnoresDisabledCache(t *testing.T) {
	app := &App{webViewPreviewData: `{"url":"https://example.com","cacheDisabled":true}`}
	if app.hasCacheableWebViewPreviewLocked() {
		t.Fatal("cache-disabled WebView previews must not retain secondary windows")
	}
}

func TestSecondaryLauncherIgnoresGlobalFocusLoss(t *testing.T) {
	if err := (&App{}).notifyFocusLost(); err != nil {
		t.Fatalf("secondary launcher should ignore global focus loss: %v", err)
	}
}

type queryFocusLifecycleTestServices struct {
	contract.Services
	calls chan struct{}
}

func (s *queryFocusLifecycleTestServices) QueryBoxFocused(context.Context, string) error {
	s.calls <- struct{}{}
	return nil
}

func TestLauncherReactivationNotifiesRetainedQueryFocus(t *testing.T) {
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return woxwidget.EditableText{
			Key:       launcherview.LauncherQueryInputKey,
			Autofocus: true,
			Child:     woxwidget.Container{Width: 100, Height: 30},
		}
	})
	host.AttachServices(formTableHostServices{})
	defer host.Dispose()
	var displayList woxui.DisplayList
	host.Frame(&displayList, woxui.FrameInfo{Size: woxui.Size{Width: 100, Height: 30}, PixelSize: woxui.PixelSize{Width: 100, Height: 30}, Scale: 1})
	if !host.HasFocus(launcherview.LauncherQueryInputKey) {
		t.Fatal("query input should retain logical focus")
	}

	services := &queryFocusLifecycleTestServices{calls: make(chan struct{}, 2)}
	app := &App{visible: true, host: host, services: services}
	app.onFocus(woxui.FocusEvent{Active: true})
	select {
	case <-services.calls:
	case <-time.After(time.Second):
		t.Fatal("window activation did not notify retained query focus")
	}

	app.onFocus(woxui.FocusEvent{Active: true})
	select {
	case <-services.calls:
		t.Fatal("repeated active event duplicated query focus notification")
	case <-time.After(20 * time.Millisecond):
	}

	app.onFocus(woxui.FocusEvent{Active: false})
	app.onFocus(woxui.FocusEvent{Active: true})
	select {
	case <-services.calls:
	case <-time.After(time.Second):
		t.Fatal("reactivation after blur did not notify retained query focus")
	}
}

func TestQueryCanFocusWhileChatPreviewIsActive(t *testing.T) {
	app := &App{}
	if !app.queryCanFocus() {
		t.Fatal("query input should own focus without an active overlay")
	}
	app.chatPreview = &chatPreviewState{active: true}
	if !app.queryCanFocus() {
		t.Fatal("active chat input prevented the query from accepting pointer focus")
	}
}

func TestQueryCanFocusWhileTerminalSearchIsOpen(t *testing.T) {
	app := &App{terminalPreview: &terminalPreviewState{SearchOpen: true}}
	if !app.queryCanFocus() {
		t.Fatal("terminal search prevented pointer focus from returning to the query input")
	}
}

func TestTerminalSearchIgnoresKeyUp(t *testing.T) {
	app := &App{terminalPreview: &terminalPreviewState{SearchOpen: true, Matches: []terminalMatch{{start: 0, end: 2}, {start: 3, end: 5}}, MatchIndex: 0}}
	if app.onTerminalPreviewKey(woxui.KeyEvent{Key: woxui.KeyEnter}) || app.terminalPreview.MatchIndex != 0 {
		t.Fatal("terminal search advanced on Enter key-up")
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

func TestMoveSelectionWrapsPastLeadingGroup(t *testing.T) {
	app := &App{
		selected: 2,
		results:  []queryResult{{ID: "group", IsGroup: true}, {ID: "first"}, {ID: "last"}},
		uiCall:   func(func()) error { return nil },
	}

	app.moveSelection(1)

	if app.selected != 1 {
		t.Fatalf("wrapped selected index = %d, want 1", app.selected)
	}
}

func TestHotkeyMatchesOnlyKeyDown(t *testing.T) {
	if hotkeyMatches("", woxui.KeyEvent{Key: woxui.KeyUnknown, Down: true}) {
		t.Fatal("unknown key unexpectedly matched an empty hotkey")
	}
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
	searchHotkey := primaryHotkey("shift+f")
	primaryModifier := woxui.KeyModifierControl
	if strings.HasPrefix(searchHotkey, "command+") {
		primaryModifier = woxui.KeyModifierMeta
	}
	if !hotkeyMatches(searchHotkey, woxui.KeyEvent{Key: "f", Modifiers: woxui.KeyModifierShift | primaryModifier, Down: true}) {
		t.Fatalf("%s did not match terminal search shortcut", searchHotkey)
	}
	if hotkeyMatches(searchHotkey, woxui.KeyEvent{Key: "f", Modifiers: primaryModifier, Down: true}) {
		t.Fatalf("%s unexpectedly accepted the shortcut without Shift", searchHotkey)
	}
}

func TestPreviousQueryHistoryStartsAtLatestInFreshMode(t *testing.T) {
	app := &App{
		queryHistories:    []plainQuery{newInputQuery("latest"), newInputQuery("older")},
		queryHistoryIndex: -1,
		canRecallHistory:  true,
	}

	query, handled := app.previousQueryHistory()
	if !handled || query == nil || query.QueryText != "latest" {
		t.Fatalf("first recalled query = %#v, handled = %v, want latest", query, handled)
	}
	query, handled = app.previousQueryHistory()
	if !handled || query == nil || query.QueryText != "older" {
		t.Fatalf("second recalled query = %#v, handled = %v, want older", query, handled)
	}
	query, handled = app.previousQueryHistory()
	if !handled || query != nil {
		t.Fatalf("exhausted history query = %#v, handled = %v, want nil and handled", query, handled)
	}
}

func TestPreviousQueryHistorySkipsCurrentQueryInContinueMode(t *testing.T) {
	app := &App{
		queryHistories:    []plainQuery{newInputQuery("current"), newInputQuery("previous")},
		queryHistoryIndex: 0,
		canRecallHistory:  true,
	}

	query, handled := app.previousQueryHistory()
	if !handled || query == nil || query.QueryText != "previous" {
		t.Fatalf("recalled query = %#v, handled = %v, want previous", query, handled)
	}
	app.canRecallHistory = false
	query, handled = app.previousQueryHistory()
	if handled || query != nil {
		t.Fatalf("disabled history query = %#v, handled = %v, want nil and unhandled", query, handled)
	}
}

func TestQueryTextChangeDisablesHistoryRecall(t *testing.T) {
	app := &App{query: newInputQuery(""), canRecallHistory: true, selected: -1}

	app.applyQueryTextChangeLocked("typed")

	if app.canRecallHistory {
		t.Fatal("history recall remained enabled after query text changed")
	}
}

func TestRequestMRUPreservesGlance(t *testing.T) {
	item := &glanceItem{Text: "100 MB"}
	uiDepth := 0
	app := &App{
		services:      requestMRUTestServices{},
		lifecycleCtx:  context.Background(),
		editor:        woxui.NewTextEditor("query"),
		glanceItem:    item,
		selected:      -1,
		themeSettings: newThemeSettingsController(CommonDeps{}),
		uiCall: func(fn func()) error {
			uiDepth++
			defer func() { uiDepth-- }()
			if uiDepth == 1 {
				fn()
			}
			return nil
		},
	}

	if err := app.requestMRU(); err != nil {
		t.Fatalf("request MRU: %v", err)
	}
	if app.glanceItem != item {
		t.Fatal("MRU request cleared the visible Glance item")
	}
}

func TestRequestMRUPreservesVisibleResultsDuringTransition(t *testing.T) {
	app := &App{
		services:       requestMRUTestServices{},
		lifecycleCtx:   context.Background(),
		editor:         woxui.NewTextEditor("query"),
		show:           showAppParams{StartPage: "mru"},
		visible:        true,
		query:          newInputQuery("old"),
		results:        []queryResult{{ID: "old-result", QueryID: "old-query"}},
		resultsQueryID: "old-query",
		selected:       0,
		themeSettings:  newThemeSettingsController(CommonDeps{}),
	}

	oldQueryID := app.query.QueryID
	if err := app.requestMRU(); err != nil {
		t.Fatalf("request MRU: %v", err)
	}
	if app.queryTransitionTimer == nil {
		t.Fatal("MRU request did not start result transition timer")
	}
	app.queryTransitionTimer.Stop()
	app.queryTransitionTimer = nil

	if app.query.QueryID == oldQueryID {
		t.Fatal("MRU request reused the previous query ID")
	}
	if len(app.results) != 1 || app.results[0].ID != "old-result" || app.selected != 0 {
		t.Fatalf("MRU transition state = results %#v selected %d, want previous result selected", app.results, app.selected)
	}
}

type sendQueryRecorderServices struct {
	contract.Services
	startedQuery common.PlainQuery
	mruCalled    bool
}

func (s *sendQueryRecorderServices) StartQuery(_ context.Context, request contract.QueryRequest, _ contract.QueryView) error {
	s.startedQuery = request.Query
	return nil
}

func (s *sendQueryRecorderServices) QueryMRU(context.Context, string, string) ([]plugin.QueryResultUI, error) {
	s.mruCalled = true
	return nil, errors.New("stop after request")
}

func newSendQueryTestApp(services *sendQueryRecorderServices, query plainQuery, show showAppParams) *App {
	return &App{
		services:        services,
		lifecycleCtx:    context.Background(),
		editor:          woxui.NewTextEditor(query.QueryText),
		generalSettings: newGeneralSettingsController(CommonDeps{}, newSharedEditState()),
		themeSettings:   newThemeSettingsController(CommonDeps{}),
		query:           query,
		show:            show,
	}
}

func waitForMRUCalled(t *testing.T, services *sendQueryRecorderServices) bool {
	t.Helper()
	deadline := time.Now().Add(200 * time.Millisecond)
	for !services.mruCalled && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	return services.mruCalled
}

func TestSendCurrentQueryPreservesSelectionQuery(t *testing.T) {
	services := &sendQueryRecorderServices{}
	app := newSendQueryTestApp(services, plainQuery{
		QueryID:        "sel",
		QueryType:      "selection",
		QuerySelection: selection{Type: "text", Text: "selected text"},
	}, showAppParams{StartPage: "mru", ShowSource: "selection", LaunchMode: "continue"})

	if err := app.sendCurrentQuery(); err != nil {
		t.Fatalf("send current query: %v", err)
	}
	if services.startedQuery.QueryType != "selection" {
		t.Fatalf("started query type = %q, want selection", services.startedQuery.QueryType)
	}
	if app.query.QueryType != "selection" || app.query.QuerySelection.Text != "selected text" {
		t.Fatalf("selection query was not preserved: %+v", app.query)
	}
	if waitForMRUCalled(t, services) {
		t.Fatal("selection query was replaced by MRU results")
	}
}

func TestSendCurrentQueryLoadsMRUForEmptyInput(t *testing.T) {
	services := &sendQueryRecorderServices{}
	app := newSendQueryTestApp(services, newInputQuery(""), showAppParams{StartPage: "mru", ShowSource: "default", LaunchMode: "continue"})

	if err := app.sendCurrentQuery(); err != nil {
		t.Fatalf("send current query: %v", err)
	}
	if !waitForMRUCalled(t, services) {
		t.Fatal("empty input with mru start page should request MRU")
	}
}

func TestShouldPreserveQueryOnShowLocked(t *testing.T) {
	selectionQuery := plainQuery{QueryType: "selection", QuerySelection: selection{Type: "text", Text: "selected"}}
	tests := []struct {
		name  string
		query plainQuery
		show  showAppParams
		want  bool
	}{
		{name: "selection show source", query: selectionQuery, show: showAppParams{ShowSource: "selection"}, want: true},
		{name: "query hotkey show source", query: selectionQuery, show: showAppParams{ShowSource: "query_hotkey"}, want: true},
		{name: "tray query show source", query: selectionQuery, show: showAppParams{ShowSource: "tray_query"}, want: true},
		{name: "explorer show source", query: selectionQuery, show: showAppParams{ShowSource: "explorer"}, want: true},
		{name: "continue selection query", query: selectionQuery, show: showAppParams{LaunchMode: "continue"}, want: true},
		{name: "continue input query with text", query: newInputQuery("abc"), show: showAppParams{LaunchMode: "continue"}, want: true},
		{name: "continue empty input query", query: newInputQuery(""), show: showAppParams{LaunchMode: "continue"}, want: false},
		{name: "default empty input query", query: newInputQuery(""), show: showAppParams{}, want: false},
	}
	for _, test := range tests {
		app := newSendQueryTestApp(&sendQueryRecorderServices{}, test.query, test.show)
		if got := app.shouldPreserveQueryOnShowLocked(); got != test.want {
			t.Fatalf("%s: shouldPreserveQueryOnShowLocked() = %v, want %v", test.name, got, test.want)
		}
	}
}

// TestApplyLaunchModeOnShowLocked covers fresh reset and explicit incoming-query preservation.
func TestApplyLaunchModeOnShowLocked(t *testing.T) {
	tests := []struct {
		name            string
		show            showAppParams
		wantPreserved   bool
		wantQueryText   string
		wantResultCount int
	}{
		{name: "fresh clears stale default query", show: showAppParams{LaunchMode: "fresh", ShowSource: "default"}, wantQueryText: "", wantResultCount: 0},
		{name: "fresh preserves injected query", show: showAppParams{LaunchMode: "fresh", ShowSource: "query_hotkey"}, wantPreserved: true, wantQueryText: "stale", wantResultCount: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newSendQueryTestApp(&sendQueryRecorderServices{}, newInputQuery("stale"), test.show)
			app.results = []queryResult{{ID: "stale-result"}}

			if got := app.applyLaunchModeOnShowLocked(); got != test.wantPreserved {
				t.Fatalf("applyLaunchModeOnShowLocked() = %v, want %v", got, test.wantPreserved)
			}
			if app.query.QueryText != test.wantQueryText || app.editor.State().Text != test.wantQueryText {
				t.Fatalf("query text = %q editor text = %q, want %q", app.query.QueryText, app.editor.State().Text, test.wantQueryText)
			}
			if len(app.results) != test.wantResultCount {
				t.Fatalf("result count = %d, want %d", len(app.results), test.wantResultCount)
			}
		})
	}
}

// TestApplyLaunchModeOnShowLockedContinuesPreviousQuery verifies continue mode retains the complete visible query state.
func TestApplyLaunchModeOnShowLockedContinuesPreviousQuery(t *testing.T) {
	query := newInputQuery("continued query")
	query.QueryID = "continued-query-id"
	app := newSendQueryTestApp(&sendQueryRecorderServices{}, query, showAppParams{LaunchMode: "continue", ShowSource: "default"})
	app.results = []queryResult{{ID: "continued-result", QueryID: query.QueryID}}
	app.resultsQueryID = query.QueryID
	app.selected = 0

	if !app.applyLaunchModeOnShowLocked() {
		t.Fatal("continue mode should preserve the previous query")
	}
	if app.query.QueryID != query.QueryID || app.query.QueryText != query.QueryText || app.editor.State().Text != query.QueryText {
		t.Fatalf("continued query state = id %q query %q editor %q, want id %q text %q", app.query.QueryID, app.query.QueryText, app.editor.State().Text, query.QueryID, query.QueryText)
	}
	if len(app.results) != 1 || app.results[0].ID != "continued-result" || app.resultsQueryID != query.QueryID || app.selected != 0 {
		t.Fatalf("continued result state = results %#v query id %q selected %d", app.results, app.resultsQueryID, app.selected)
	}
}

func TestInformationalGlanceCanBeTapped(t *testing.T) {
	app := &App{}
	widget := app.buildGlance(glanceItem{Text: "100 MB"}, true, defaultPalette(), 100, 1, launcherDensityMetricsFor(""))
	boundary, ok := widget.(woxwidget.Boundary[launcherview.GlanceProps])
	if !ok {
		t.Fatalf("Glance widget = %T, want boundary", widget)
	}
	if boundary.Props.OnTap == nil {
		t.Fatal("informational Glance should expose a refresh tap handler")
	}
}

func TestUpdatedResultBoundaryKeysFollowFieldUpdates(t *testing.T) {
	title := "Title"
	subtitle := "Subtitle"
	icon := common.WoxImage{}
	tails := []plugin.QueryResultTail{}
	update := plugin.UpdatableResult{Id: "live", Icon: &icon, Title: &title, SubTitle: &subtitle, Tails: &tails}
	got := updatedResultBoundaryKeys(update, false)
	want := []woxwidget.Key{
		launcherview.LauncherResultIconBoundaryKey("live"),
		launcherview.LauncherResultTitleBoundaryKey("live"),
		launcherview.LauncherResultSubtitleBoundaryKey("live"),
		launcherview.LauncherResultTailsBoundaryKey("live"),
	}
	if len(got) != len(want) {
		t.Fatalf("updated result boundary keys = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("updated result boundary keys = %v, want %v", got, want)
		}
	}
	grid := updatedResultBoundaryKeys(update, true)
	if len(grid) != 2 || grid[0] != want[0] || grid[1] != want[1] {
		t.Fatalf("grid updated result boundary keys = %v, want icon and title only", grid)
	}
}

func TestResultPreviewBecameVisible(t *testing.T) {
	preview := &plugin.WoxPreview{PreviewType: "markdown", PreviewData: "translated text"}
	if !resultPreviewBecameVisible(queryPreview{}, preview) {
		t.Fatal("empty preview should require bounds refresh when content first arrives")
	}
	if resultPreviewBecameVisible(queryPreview{PreviewData: "first chunk"}, preview) {
		t.Fatal("subsequent streaming preview updates should not recalculate window bounds")
	}
	if resultPreviewBecameVisible(queryPreview{}, nil) {
		t.Fatal("missing preview update should not recalculate window bounds")
	}
}

func TestMediaPreviewBypassesPreparedSectionBoundary(t *testing.T) {
	app := &App{}
	result := queryResult{Preview: queryPreview{PreviewType: "media", PreviewData: `{"title":"Track"}`}}
	widget := app.buildPreviewSection(result, viewSnapshot{palette: defaultPalette()}, 700, 400, 1, 0)
	if _, wrapped := widget.(woxwidget.Boundary[launcherPreparedSectionProps]); wrapped {
		t.Fatal("media preview retained the full-section boundary")
	}
}

func TestUnresolvedRemotePreviewRendersBlank(t *testing.T) {
	app := &App{remotePreviews: map[string]queryPreview{}}
	widget := app.buildPreview(queryResult{Preview: queryPreview{PreviewType: "remote", PreviewData: "/preview?id=result"}}, defaultPalette(), 700, 400, 1, 0)
	blank, ok := widget.(woxwidget.Container)
	if !ok || blank.Child != nil || blank.Width != 700 || blank.Height != 400 {
		t.Fatalf("unresolved remote preview = %#v, want blank 700x400 container", widget)
	}
}

func TestPrepareRemotePreviewStartsDeferredRequest(t *testing.T) {
	path := "/preview?sessionId=session&queryId=query&id=result"
	services := &remotePreviewTestServices{called: make(chan struct{})}
	app := &App{
		services:        services,
		lifecycleCtx:    context.Background(),
		remotePreviews:  map[string]queryPreview{},
		previewRequests: map[string]bool{},
	}

	app.prepareRemotePreview(queryPreview{PreviewType: "remote", PreviewData: path})
	select {
	case <-services.called:
	case <-time.After(time.Second):
		t.Fatal("deferred remote preview request did not start")
	}
	if !app.previewRequests[path] {
		t.Fatal("remote preview request was not marked as pending")
	}
}

func TestQueryEditorShiftEnterInsertsNewline(t *testing.T) {
	editor := woxui.NewTextEditor("firstsecond")
	editor.SetCaret(5)
	handled, changed := handleQueryEditorKey(editor, woxui.KeyEvent{Key: woxui.KeyEnter, Modifiers: woxui.KeyModifierShift, Down: true})
	if !handled || !changed || editor.State().Text != "first\nsecond" {
		t.Fatalf("Shift+Enter = handled %v, changed %v, text %q", handled, changed, editor.State().Text)
	}
	if handled, changed = handleQueryEditorKey(editor, woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}); handled || changed {
		t.Fatalf("plain Enter = handled %v, changed %v, want launcher activation", handled, changed)
	}
}

func TestNormalizeQueryNewlinesPreservesPastedLines(t *testing.T) {
	if got := normalizeQueryNewlines("one\r\ntwo\rthree"); got != "one\ntwo\nthree" {
		t.Fatalf("normalized pasted text = %q", got)
	}
}

func TestQueryPasteReplacesSelectionWithMultilineText(t *testing.T) {
	editor := woxui.NewTextEditor("replace me")
	editor.SelectAll()
	if !editor.InsertText(normalizeQueryNewlines("one\r\ntwo\rthree")) {
		t.Fatal("multiline paste did not change query text")
	}
	state := editor.State()
	if state.Text != "one\ntwo\nthree" || state.Selection.Focus != len([]rune(state.Text)) || !state.Selection.Collapsed() {
		t.Fatalf("pasted query state = text %q selection %#v", state.Text, state.Selection)
	}
}

func TestQueryDisplayProvidesAllLinesToSharedViewport(t *testing.T) {
	runes := []rune("one\ntwo\nthree\nfour\nfive")
	lines, caretLine, _, _, _, _, _ := queryDisplayLines(runes, len(runes), len(runes), len(runes), -1, 0, func(value string) float32 {
		return float32(len([]rune(value)))
	})
	if len(lines) != 5 || caretLine != 4 || lines[4].Text != "five" {
		t.Fatalf("query lines = count %d, caret %d, last %q", len(lines), caretLine, lines[4].Text)
	}
}

func TestFromCoreShowOptionsPreservesQueryHistoryOrderAndPayload(t *testing.T) {
	options := contract.ShowOptions{ShowPreviewTitleBar: true, QueryHistories: []common.PlainQuery{
		{QueryId: "latest-id", QueryType: "input", QueryText: "latest", QueryRefinements: map[string]string{"scope": "recent"}, ContextData: common.ContextData{"token": "latest"}},
		{QueryId: "older-id", QueryType: "selection", QueryText: "older", QuerySelection: utilselection.Selection{Text: "selected"}},
	}}

	params := fromCoreShowOptions(options)
	if !params.ShowPreviewTitleBar {
		t.Fatal("preview title-bar control was not preserved")
	}
	if len(params.QueryHistories) != 2 {
		t.Fatalf("query history count = %d, want 2", len(params.QueryHistories))
	}
	if got := params.QueryHistories[0]; got.QueryID != "latest-id" || got.QueryText != "latest" || got.QueryRefinements["scope"] != "recent" || got.ContextData["token"] != "latest" {
		t.Fatalf("latest query history = %#v, want complete latest payload", got)
	}
	if got := params.QueryHistories[1]; got.QueryID != "older-id" || got.QueryType != "selection" || got.QuerySelection.Text != "selected" {
		t.Fatalf("older query history = %#v, want complete older payload", got)
	}
}
