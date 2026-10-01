package launcher

import (
	"testing"

	"wox/plugin"
	woxui "wox/ui/runtime"
)

// TestPreviewVisibilityLifetime covers result defaults, query-wide manual input, live updates, and query replacement.
func TestPreviewVisibilityLifetime(t *testing.T) {
	app := &App{
		hotkeySettings: newHotkeySettingsController(CommonDeps{}),
		query:          plainQuery{QueryID: "query"}, resultsQueryID: "query", selected: 0,
		results: []queryResult{
			{ID: "hidden", Preview: fromCorePreview(plugin.WoxPreview{PreviewType: "text", PreviewData: "first", DefaultHidden: true})},
			{ID: "normal", Preview: queryPreview{PreviewType: "text", PreviewData: "second"}},
			{ID: "empty"},
		},
	}
	visible := func() bool {
		return launcherPreviewVisible(app.selectedPreviewLayout(), app.results[app.selected].Preview)
	}
	app.reconcileSelectedPreview()
	if visible() {
		t.Fatal("hidden result ignored its default")
	}
	app.selected = 1
	app.reconcileSelectedPreview()
	if !visible() {
		t.Fatal("selection inherited another result's default before manual input")
	}
	app.selected = 0
	app.reconcileSelectedPreview()
	modifier := woxui.KeyModifierControl
	if primaryHotkey("p") == "command+p" {
		modifier = woxui.KeyModifierMeta
	}
	if visible() || !app.onKey(woxui.KeyEvent{Key: "p", Down: true, Modifiers: modifier}) || !visible() {
		t.Fatal("hidden preview did not open manually")
	}
	app.results[0].Preview.PreviewData = "updated"
	app.reconcileSelectedPreview()
	if !visible() {
		t.Fatal("same-result update reset manual visibility")
	}
	app.selected = 1
	app.reconcileSelectedPreview()
	if !visible() || !app.toggleSelectedPreview() || visible() {
		t.Fatal("normal preview did not open by default or close manually")
	}
	app.selected = 0
	app.reconcileSelectedPreview()
	if visible() {
		t.Fatal("manual closing was lost when selection changed")
	}
	app.selected = 1
	app.reconcileSelectedPreview()
	if visible() {
		t.Fatal("a normally visible result ignored query-wide closing")
	}
	app.selected = 2
	app.reconcileSelectedPreview()
	if visible() || app.toggleSelectedPreview() {
		t.Fatal("a result without preview content showed or toggled a preview")
	}
	app.selected = -1
	app.reconcileSelectedPreview()
	app.selected = 1
	app.reconcileSelectedPreview()
	if visible() || !app.toggleSelectedPreview() {
		t.Fatal("temporary absence of selection lost the query's manual choice")
	}
	app.selected = 0
	app.reconcileSelectedPreview()
	if !visible() {
		t.Fatal("a normally hidden result ignored query-wide opening")
	}
	app.query.QueryID, app.resultsQueryID = "next", "next"
	app.reconcileSelectedPreview()
	if visible() {
		t.Fatal("a new query reused the old query override")
	}
	app.selected = 1
	app.reconcileSelectedPreview()
	if !visible() {
		t.Fatal("a new query did not restore the normally visible result's default")
	}
}

// TestHiddenPreviewDefersResources ensures a hidden remote payload starts no request and does not reopen on updates.
func TestHiddenPreviewDefersResources(t *testing.T) {
	app := &App{
		visible: true, query: plainQuery{QueryID: "q"}, resultsQueryID: "q", selected: 0,
		results: []queryResult{{ID: "r", Preview: queryPreview{PreviewType: "remote", PreviewData: "/preview?id=r", DefaultHidden: true}}},
	}
	app.reconcileSelectedPreview()
	if _, _, visible := app.selectedPreviewForLifecycle(); visible || len(app.previewRequests) != 0 {
		t.Fatal("hidden preview started resource loading")
	}
	hidden := false
	app.previewVisibility.visible = &hidden
	app.results[0].Preview.DefaultHidden = false
	app.reconcileSelectedPreview()
	if launcherPreviewVisible(app.selectedPreviewLayout(), app.results[0].Preview) {
		t.Fatal("a live update overrode the query's manual visibility")
	}
	app.webViewFullscreen = true
	if !launcherPreviewVisible(app.selectedPreviewLayout(), app.results[0].Preview) {
		t.Fatal("explicit fullscreen mode was suppressed by a collapsed row")
	}
}

// TestPreviewVisibilityLayoutOverride preserves configured ratios while allowing manual opening of a list-only layout.
func TestPreviewVisibilityLayoutOverride(t *testing.T) {
	ratio := 1.0
	app := &App{
		query: plainQuery{QueryID: "q"}, resultsQueryID: "q", selected: 0,
		layout:  queryLayout{ResultPreviewWidthRatio: &ratio},
		results: []queryResult{{ID: "r", Preview: queryPreview{PreviewType: "remote", PreviewData: "/preview?id=r"}}},
	}
	if launcherPreviewVisible(app.selectedPreviewLayout(), app.results[0].Preview) {
		t.Fatal("list-only layout showed a preview automatically")
	}
	if !app.toggleSelectedPreview() || launcherPreviewRatio(app.selectedPreviewLayout(), false) != 0.4 || ratio != 1 {
		t.Fatal("manual opening did not use a temporary split layout")
	}
}
