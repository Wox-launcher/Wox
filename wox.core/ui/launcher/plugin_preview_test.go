package launcher

import (
	"context"
	"testing"

	"wox/plugin"
	"wox/ui/contract"
	"wox/util"
)

func TestPluginPreviewContextLifecycle(t *testing.T) {
	var state pluginPreviewState
	first := queryPreview{PreviewType: "text", PreviewData: "first"}
	second := queryPreview{PreviewType: "text", PreviewData: "second"}
	context := queryContext{PluginID: "radio"}
	state.accept(context, &first, "query-one")
	state.accept(context, &second, "query-two")
	if state.preview == nil || state.preview.PreviewData != "first" {
		t.Fatal("query response replaced the active plugin preview")
	}
	state.accept(context, nil, "query-three")
	if state.preview == nil {
		t.Fatal("missing preview cleared the active plugin preview")
	}
	if result, ok := state.result(); !ok || result.QueryID != "query-one" {
		t.Fatal("preview identity changed with query")
	}
	state.accept(queryContext{PluginID: "other"}, &second, "other-query")
	if state.preview == nil || state.preview.PreviewData != "second" {
		t.Fatal("new plugin context did not replace the old preview")
	}
	state.accept(queryContext{IsGlobalQuery: true}, nil, "global")
	if state.preview != nil || state.pluginID != "" {
		t.Fatal("leaving the plugin context did not release the preview")
	}
}

func TestPluginPreviewRejectsStaleUpdates(t *testing.T) {
	var state pluginPreviewState
	first := queryPreview{PreviewType: "woxview", PreviewData: "first", WoxViewID: "doc"}
	changed := queryPreview{PreviewType: "woxview", PreviewData: "changed", WoxViewID: "doc"}
	state.accept(queryContext{PluginID: "radio"}, &first, "query")
	if state.updateWoxView("other", "doc", changed) || state.updateWoxView("radio", "old", changed) {
		t.Fatal("accepted update from a different owner or document")
	}
	if !state.updateWoxView("radio", "doc", changed) || state.preview.PreviewData != "changed" {
		t.Fatal("rejected current document update")
	}
	state.clear()
	if state.updateWoxView("radio", "doc", changed) {
		t.Fatal("accepted an update after the context was released")
	}
}

func TestPluginPreviewTakesPrecedenceOverSelectedResult(t *testing.T) {
	app := bindTestApp(&App{
		visible:      true,
		querySession: queryController{query: plainQuery{QueryID: "new"}},
		resultList: resultController{resultsQueryID: "new", selected: 0, results: []queryResult{
			{ID: "row", Preview: queryPreview{PreviewType: "text", PreviewData: "row preview", DefaultHidden: true}},
		}},
	})
	preview := queryPreview{PreviewType: "text", PreviewData: "plugin preview"}
	app.previews.pluginPreview.accept(queryContext{PluginID: "radio"}, &preview, "old")
	result, got, visible := app.previews.selectedPreviewForLifecycle()
	if !visible || got.PreviewData != "plugin preview" || result.QueryID != "old" {
		t.Fatalf("plugin preview was not retained: %#v %#v %t", result, got, visible)
	}
	app.resultList.results = nil
	app.resultList.selected = -1
	if _, _, visible := app.previews.selectedPreviewForLifecycle(); !visible {
		t.Fatal("plugin preview disappeared when results became empty")
	}
}

// TestRequestMRUClearsRadioPluginPreview checks teardown before and after the MRU rows arrive.
func TestRequestMRUClearsRadioPluginPreview(t *testing.T) {
	app := newRetainedMRUTestApp(t)
	app.querySession.query = newInputQuery("radio ")
	app.querySession.editor.SetText("radio ", false)
	app.ApplyQueryResponse(context.Background(), contract.QueryResponse{
		QueryID: app.querySession.query.QueryID,
		IsFinal: true,
		Response: plugin.QueryResponseUI{
			Context: plugin.QueryContext{PluginId: "radio"},
			Results: []plugin.QueryResultUI{{Id: "station", Title: "FM106.2"}},
			PluginPreview: &plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeWoxView,
				PreviewData: `{"version":1,"root":{"type":"text","props":{"text":"FM106.2"}}}`,
				WoxViewID:   "radio-player",
			},
		},
	})
	radio := app.snapshot().pluginPreview
	if radio == nil {
		t.Fatal("Radio query did not install a plugin preview")
	}
	app.buildWoxViewPreview(*radio, "radio-preview", radio.Preview, uiPalette{}, 300, 200)
	if app.previews.woxView == nil {
		t.Fatal("Radio preview did not mount its WoxView")
	}
	t.Cleanup(app.previews.clearWoxViewPreview)

	if err := app.requestMRU(); err != nil {
		t.Fatalf("request MRU: %v", err)
	}
	app.querySession.resetQueryTransitionLocked()
	if app.snapshot().pluginPreview != nil || app.previews.woxView != nil {
		t.Fatal("switching to MRU kept the Radio preview mounted while waiting for results")
	}

	mruQueryID := app.querySession.query.QueryID
	app.applyResults(mruQueryID, []queryResult{{ID: "settings", Title: "Open Wox Settings"}}, &queryLayout{}, nil, &queryContext{IsGlobalQuery: true}, 0, true)
	snapshot := app.snapshot()
	if len(snapshot.results) != 1 || snapshot.results[0].ID != "settings" || snapshot.pluginPreview != nil {
		t.Fatal("MRU did not replace the Radio results with a preview-free settings row")
	}
}

func TestUpdatePluginPreviewValidatesOwnerSessionAndTree(t *testing.T) {
	app := bindTestApp(&App{sessionID: "window"})
	initial := queryPreview{PreviewType: "woxview", PreviewData: `{"version":1,"root":{"type":"text","props":{"text":"old"}}}`, WoxViewID: "doc"}
	app.previews.pluginPreview.accept(queryContext{PluginID: "radio"}, &initial, "old-query")
	updated := plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeWoxView, PreviewData: `{"version":1,"root":{"type":"text","props":{"text":"new"}}}`, WoxViewID: "doc"}
	ctx := util.WithQueryIdContext(util.WithSessionContext(context.Background(), "window"), "new-query")
	call := func(ctx context.Context, owner string, p plugin.WoxPreview) bool {
		result, err := app.UpdatePluginPreview(ctx, owner, plugin.UpdatePluginPreviewOption{Preview: p})
		if err != nil {
			t.Fatal(err)
		}
		return result.Success
	}
	if call(ctx, "other", updated) || call(context.Background(), "radio", updated) {
		t.Fatal("accepted foreign owner or missing session")
	}
	invalid := updated
	invalid.PreviewData = "not JSON"
	if call(ctx, "radio", invalid) {
		t.Fatal("accepted invalid WoxView tree")
	}
	if !call(ctx, "radio", updated) || app.previews.pluginPreview.preview.PreviewData != updated.PreviewData {
		t.Fatal("rejected current plugin document after query change")
	}
}
