package launcher

import (
	"testing"

	"wox/common"
	"wox/ui/contract"
	woxui "wox/ui/runtime"
	utilselection "wox/util/selection"
)

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
