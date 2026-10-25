package launcher

import (
	"testing"

	"wox/plugin"
	woxui "wox/ui/runtime"
)

// TestQueryResultKeepsDefaultHidden prevents the launcher conversion from opening a collapsed preview.
func TestQueryResultKeepsDefaultHidden(t *testing.T) {
	result := plugin.QueryResult{Preview: plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeFile, PreviewData: "/tmp", DefaultHidden: true}}
	converted := fromCoreQueryResult(result.ToUI())
	if !converted.Preview.DefaultHidden {
		t.Fatal("default-hidden preview was lost between plugin and launcher")
	}
	app := bindTestApp(&App{
		querySession: queryController{query: plainQuery{QueryID: "query"}},
		resultList: resultController{
			resultsQueryID: "query",
			selected:       0,
			results:        []queryResult{converted},
		},
	})
	if launcherPreviewVisible(app.previews.selectedPreviewLayout(), app.resultList.results[0].Preview) {
		t.Fatal("default-hidden file preview opened on selection")
	}
}

// TestPreviewVisibilityLifetime covers result defaults, query-wide manual input, live updates, and query replacement.
func TestPreviewVisibilityLifetime(t *testing.T) {
	app := bindTestApp(&App{
		hotkeySettings: newHotkeySettingsController(CommonDeps{}),
		querySession:   queryController{query: plainQuery{QueryID: "query"}},
		resultList: resultController{
			resultsQueryID: "query",
			selected:       0,
			results: []queryResult{
				{ID: "hidden", Preview: fromCorePreview(plugin.WoxPreview{PreviewType: "text", PreviewData: "first", DefaultHidden: true})},
				{ID: "normal", Preview: queryPreview{PreviewType: "text", PreviewData: "second"}},
				{ID: "empty"},
			},
		},
	})
	visible := func() bool {
		return launcherPreviewVisible(app.previews.selectedPreviewLayout(), app.resultList.results[app.resultList.selected].Preview)
	}
	app.previews.reconcileSelectedPreview()
	if visible() {
		t.Fatal("hidden result ignored its default")
	}
	app.resultList.selected = 1
	app.previews.reconcileSelectedPreview()
	if !visible() {
		t.Fatal("selection inherited another result's default before manual input")
	}
	app.resultList.selected = 0
	app.previews.reconcileSelectedPreview()
	modifier := woxui.KeyModifierControl
	if primaryHotkey("p") == "command+p" {
		modifier = woxui.KeyModifierMeta
	}
	if visible() || !app.onKey(woxui.KeyEvent{Key: "p", Down: true, Modifiers: modifier}) || !visible() {
		t.Fatal("hidden preview did not open manually")
	}
	app.resultList.results[0].Preview.PreviewData = "updated"
	app.previews.reconcileSelectedPreview()
	if !visible() {
		t.Fatal("same-result update reset manual visibility")
	}
	app.resultList.selected = 1
	app.previews.reconcileSelectedPreview()
	if !visible() || !app.toggleSelectedPreview() || visible() {
		t.Fatal("normal preview did not open by default or close manually")
	}
	app.resultList.selected = 0
	app.previews.reconcileSelectedPreview()
	if visible() {
		t.Fatal("manual closing was lost when selection changed")
	}
	app.resultList.selected = 1
	app.previews.reconcileSelectedPreview()
	if visible() {
		t.Fatal("a normally visible result ignored query-wide closing")
	}
	app.resultList.selected = 2
	app.previews.reconcileSelectedPreview()
	if visible() || app.toggleSelectedPreview() {
		t.Fatal("a result without preview content showed or toggled a preview")
	}
	app.resultList.selected = -1
	app.previews.reconcileSelectedPreview()
	app.resultList.selected = 1
	app.previews.reconcileSelectedPreview()
	if visible() || !app.toggleSelectedPreview() {
		t.Fatal("temporary absence of selection lost the query's manual choice")
	}
	app.resultList.selected = 0
	app.previews.reconcileSelectedPreview()
	if !visible() {
		t.Fatal("a normally hidden result ignored query-wide opening")
	}
	app.querySession.query.QueryID, app.resultList.resultsQueryID = "next", "next"
	app.previews.reconcileSelectedPreview()
	if visible() {
		t.Fatal("a new query reused the old query override")
	}
	app.resultList.selected = 1
	app.previews.reconcileSelectedPreview()
	if !visible() {
		t.Fatal("a new query did not restore the normally visible result's default")
	}
}

// TestHiddenPreviewDefersResources ensures a hidden remote payload starts no request and does not reopen on updates.
func TestHiddenPreviewDefersResources(t *testing.T) {
	app := bindTestApp(&App{
		visible:      true,
		querySession: queryController{query: plainQuery{QueryID: "q"}},
		resultList: resultController{
			resultsQueryID: "q",
			selected:       0,
			results:        []queryResult{{ID: "r", Preview: queryPreview{PreviewType: "remote", PreviewData: "/preview?id=r", DefaultHidden: true}}},
		},
	})
	app.previews.reconcileSelectedPreview()
	if _, _, visible := app.previews.selectedPreviewForLifecycle(); visible || len(app.remotePreview.previewRequests) != 0 {
		t.Fatal("hidden preview started resource loading")
	}
	hidden := false
	app.previews.previewVisibility.visible = &hidden
	app.resultList.results[0].Preview.DefaultHidden = false
	app.previews.reconcileSelectedPreview()
	if launcherPreviewVisible(app.previews.selectedPreviewLayout(), app.resultList.results[0].Preview) {
		t.Fatal("a live update overrode the query's manual visibility")
	}
	app.webView.webViewFullscreen = true
	if !launcherPreviewVisible(app.previews.selectedPreviewLayout(), app.resultList.results[0].Preview) {
		t.Fatal("explicit fullscreen mode was suppressed by a collapsed row")
	}
}

// TestPreviewVisibilityLayoutOverride preserves configured ratios while allowing manual opening of a list-only layout.
func TestPreviewVisibilityLayoutOverride(t *testing.T) {
	ratio := 1.0
	app := bindTestApp(&App{
		querySession: queryController{query: plainQuery{QueryID: "q"}},
		resultList: resultController{
			resultsQueryID: "q",
			selected:       0,
			layout:         queryLayout{ResultPreviewWidthRatio: &ratio},
			results:        []queryResult{{ID: "r", Preview: queryPreview{PreviewType: "remote", PreviewData: "/preview?id=r"}}},
		},
	})
	if launcherPreviewVisible(app.previews.selectedPreviewLayout(), app.resultList.results[0].Preview) {
		t.Fatal("list-only layout showed a preview automatically")
	}
	if !app.toggleSelectedPreview() || launcherPreviewRatio(app.previews.selectedPreviewLayout(), false) != 0.4 || ratio != 1 {
		t.Fatal("manual opening did not use a temporary split layout")
	}
}
