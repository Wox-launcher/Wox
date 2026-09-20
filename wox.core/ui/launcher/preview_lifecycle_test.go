package launcher

import (
	"errors"
	"testing"

	woxui "wox/ui/runtime"
)

func TestSelectedPreviewForLifecycleRejectsStaleQueryResults(t *testing.T) {
	app := &App{
		visible:        true,
		query:          plainQuery{QueryID: "current-query"},
		resultsQueryID: "previous-query",
		results:        []queryResult{{Preview: queryPreview{PreviewType: "text", PreviewData: "stale"}}},
		selected:       0,
	}

	if _, _, visible := app.selectedPreviewForLifecycle(); visible {
		t.Fatal("stale query result kept its preview lifecycle active")
	}
}

func TestSelectedPreviewForLifecycleRejectsPreviewlessResult(t *testing.T) {
	app := &App{
		visible:        true,
		query:          plainQuery{QueryID: "current-query"},
		resultsQueryID: "current-query",
		results:        []queryResult{{Preview: queryPreview{PreviewType: "text"}}},
		selected:       0,
	}

	if _, _, visible := app.selectedPreviewForLifecycle(); visible {
		t.Fatal("result without preview data kept its preview lifecycle active")
	}
}

// TestWebViewPreviewSharesResultGracePeriod keeps the visible browser until results replace it or expire.
func TestWebViewPreviewSharesResultGracePeriod(t *testing.T) {
	const html = `{"html":"<p>previous result</p>"}`
	for _, kind := range []string{"webview", "remote", "file"} {
		t.Run(kind, func(t *testing.T) {
			preview := queryPreview{PreviewType: kind, PreviewData: html}
			app := &App{
				visible: true, query: plainQuery{QueryID: "new-query", QueryText: "random ip s"},
				resultsQueryID: "old-query", selected: 0, webViewPreviewData: html,
			}
			if kind == "remote" {
				preview.PreviewData = "/preview?id=old"
				app.remotePreviews = map[string]queryPreview{preview.PreviewData: {PreviewType: "webview", PreviewData: html}}
			} else if kind == "file" {
				preview.PreviewData = "preview.html"
				app.filePreviews = map[string]filePreviewContent{preview.PreviewData: {Kind: "webview", WebViewData: html}}
			}
			app.results = []queryResult{{Preview: preview}}
			app.beginQueryTransitionLocked(true)
			if app.queryTransitionTimer == nil {
				t.Fatal("result grace period was not scheduled")
			}
			// Drive the deadline explicitly without posting to a native UI event loop.
			app.queryTransitionTimer.Stop()
			defer app.resetQueryTransitionLocked()
			if _, _, visible := app.selectedPreviewForLifecycle(); !visible {
				t.Fatal("active WebView was hidden before retained results expired")
			}
			app.reconcileSelectedPreviewOnUI()
			if app.webViewPreviewData != html {
				t.Fatal("query transition deactivated the retained WebView")
			}
			app.resetQueryTransitionLocked()
			app.reconcileSelectedPreviewOnUI()
			if app.webViewPreviewData != "" {
				t.Fatal("WebView remained active after the result grace period")
			}
		})
	}
}

// TestWebViewGracePeriodDoesNotKeepUnrenderedOrNewPreviews prevents stale surfaces outliving their result.
func TestWebViewGracePeriodDoesNotKeepUnrenderedOrNewPreviews(t *testing.T) {
	const html = `{"html":"<p>old</p>"}`
	for _, change := range []string{"hidden", "destroyed", "different-preview", "error", "empty-results", "new-results", "no-preview-layout"} {
		t.Run(change, func(t *testing.T) {
			app := &App{
				visible: true, query: plainQuery{QueryID: "new-query", QueryText: "random ip s"},
				resultsQueryID: "old-query", selected: 0, webViewPreviewData: html,
				results: []queryResult{{Preview: queryPreview{PreviewType: "webview", PreviewData: html}}},
			}
			app.beginQueryTransitionLocked(true)
			app.queryTransitionTimer.Stop()
			defer app.resetQueryTransitionLocked()
			switch change {
			case "hidden":
				app.visible = false
			case "destroyed":
				app.destroyed.Store(true)
			case "different-preview":
				app.results[0].Preview.PreviewData = `{"html":"<p>different</p>"}`
			case "error":
				app.webViewPreviewError = "load failed"
			case "empty-results":
				app.results = nil
			case "new-results":
				app.resultsQueryID = app.query.QueryID
				app.results[0].Preview = queryPreview{PreviewType: "text", PreviewData: "new result"}
			case "no-preview-layout":
				ratio := float64(1)
				app.layout.ResultPreviewWidthRatio = &ratio
			}
			app.reconcileSelectedPreviewOnUI()
			if app.webViewPreviewData != "" {
				t.Fatal("unrendered WebView survived reconciliation")
			}
		})
	}
}

func TestNativeFilePreviewLifecycleAdvancesGeneration(t *testing.T) {
	app := &App{}
	changed := app.activateNativeFilePreview("first.docx")
	if !changed || app.nativeFilePreviewGeneration != 1 {
		t.Fatalf("first native preview activation = changed %v generation %d", changed, app.nativeFilePreviewGeneration)
	}

	app.deactivateNativeFilePreview()
	if app.nativeFilePreviewGeneration != 2 {
		t.Fatalf("native preview deactivation generation = %d, want 2", app.nativeFilePreviewGeneration)
	}
	changed = app.activateNativeFilePreview("second.docx")
	if !changed || app.nativeFilePreviewGeneration != 3 {
		t.Fatalf("second native preview activation = changed %v generation %d, want generation 3", changed, app.nativeFilePreviewGeneration)
	}
}

func TestNativeFilePreviewIgnoresStaleErrors(t *testing.T) {
	app := &App{nativeFilePreviewPath: "current.docx", nativeFilePreviewGeneration: 2}
	app.setNativeFilePreviewError(1, errors.New("stale preview failure"))
	if app.nativeFilePreviewError != "" || app.nativeFilePreviewGeneration != 2 {
		t.Fatalf("stale native preview error changed state: error %q generation %d", app.nativeFilePreviewError, app.nativeFilePreviewGeneration)
	}
}

func TestNativeFilePreviewDelayedActivationRejectsObsoleteSelection(t *testing.T) {
	app := &App{}
	if !app.scheduleNativeFilePreview("first.docx") {
		t.Fatal("initial native preview should schedule")
	}
	generation := app.nativeFilePreviewGeneration
	if app.nativeFilePreviewPendingPath != "first.docx" || app.nativeFilePreviewTimer == nil {
		t.Fatal("scheduled native preview did not retain its cancellation state")
	}

	app.deactivateNativeFilePreview()
	app.activateScheduledNativeFilePreview("first.docx", generation)
	if app.nativeFilePreviewPath != "" {
		t.Fatalf("obsolete delayed preview became active for %q", app.nativeFilePreviewPath)
	}
}

func TestNativeFilePreviewCoalescesPendingBounds(t *testing.T) {
	app := &App{nativeFilePreviewPath: "document.docx", nativeFilePreviewGeneration: 3}
	first := woxui.Rect{X: 1, Y: 2, Width: 300, Height: 400}
	latest := woxui.Rect{X: 2, Y: 3, Width: 320, Height: 420}
	app.requestNativeFilePreviewBounds("document.docx", 3, first)
	timer := app.nativeFilePreviewBoundsTimer
	if timer == nil {
		t.Fatal("first native preview bounds should schedule one deferred update")
	}
	app.requestNativeFilePreviewBounds("document.docx", 3, latest)
	if app.nativeFilePreviewBoundsTimer != timer {
		t.Fatal("new bounds should update the pending request instead of scheduling another native operation")
	}
	if app.nativeFilePreviewBounds != latest {
		t.Fatalf("pending native preview bounds = %+v, want %+v", app.nativeFilePreviewBounds, latest)
	}
	app.stopNativeFilePreviewTimers()
}
