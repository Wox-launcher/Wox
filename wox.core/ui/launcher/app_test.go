package launcher

import (
	"context"
	"errors"
	"strings"
	"testing"

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

func TestLauncherGridHidesRegularPreview(t *testing.T) {
	layout := queryLayout{GridLayout: &gridLayout{Columns: 4}}
	if launcherPreviewVisible(layout, queryPreview{PreviewType: "image", PreviewData: "wallpaper"}) {
		t.Fatal("grid layout should hide regular result previews")
	}
	if !launcherPreviewVisible(layout, queryPreview{PreviewType: "query_requirement_settings", PreviewData: "settings"}) {
		t.Fatal("grid layout should preserve Flutter's interactive settings preview exception")
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

func TestSecondaryLauncherHideClosesInstance(t *testing.T) {
	app := &App{}
	// Keep the test scoped to hide routing; native-close cleanup is covered by the instance lifecycle.
	app.destroyOnce.Do(func() {})
	if err := app.hideWindow(false); err != nil {
		t.Fatalf("secondary launcher hide should close the instance: %v", err)
	}
}

func TestSecondaryLauncherIgnoresGlobalFocusLoss(t *testing.T) {
	if err := (&App{}).notifyFocusLost(); err != nil {
		t.Fatalf("secondary launcher should ignore global focus loss: %v", err)
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

func TestInformationalGlanceCanBeTapped(t *testing.T) {
	app := &App{}
	widget := app.buildGlance(glanceItem{Text: "100 MB"}, true, defaultPalette(), 100, 1, launcherDensityMetricsFor(""))
	stateful, ok := widget.(woxwidget.Stateful)
	if !ok {
		t.Fatalf("Glance widget = %T, want stateful", widget)
	}
	props, ok := stateful.Widget.(launcherview.GlanceProps)
	if !ok || props.OnTap == nil {
		t.Fatal("informational Glance should expose a refresh tap handler")
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
	options := contract.ShowOptions{QueryHistories: []common.PlainQuery{
		{QueryId: "latest-id", QueryType: "input", QueryText: "latest", QueryRefinements: map[string]string{"scope": "recent"}, ContextData: common.ContextData{"token": "latest"}},
		{QueryId: "older-id", QueryType: "selection", QueryText: "older", QuerySelection: utilselection.Selection{Text: "selected"}},
	}}

	params := fromCoreShowOptions(options)
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
