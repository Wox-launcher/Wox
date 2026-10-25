package launcher

import (
	"errors"
	"testing"

	woxui "wox/ui/runtime"
)

func TestSelectedPreviewForLifecycleRejectsStaleQueryResults(t *testing.T) {
	app := bindTestApp(&App{
		visible:      true,
		querySession: queryController{query: plainQuery{QueryID: "current-query"}},
		resultList: resultController{
			resultsQueryID: "previous-query",
			results:        []queryResult{{Preview: queryPreview{PreviewType: "text", PreviewData: "stale"}}},
			selected:       0,
		},
	})

	if _, _, visible := app.previews.selectedPreviewForLifecycle(); visible {
		t.Fatal("stale query result kept its preview lifecycle active")
	}
}

func TestSelectedPreviewForLifecycleRejectsPreviewlessResult(t *testing.T) {
	app := bindTestApp(&App{
		visible:      true,
		querySession: queryController{query: plainQuery{QueryID: "current-query"}},
		resultList: resultController{
			resultsQueryID: "current-query",
			results:        []queryResult{{Preview: queryPreview{PreviewType: "text"}}},
			selected:       0,
		},
	})

	if _, _, visible := app.previews.selectedPreviewForLifecycle(); visible {
		t.Fatal("result without preview data kept its preview lifecycle active")
	}
}

// TestWebViewPreviewSharesResultGracePeriod keeps the visible browser until results replace it or expire.
func TestWebViewPreviewSharesResultGracePeriod(t *testing.T) {
	const html = `{"html":"<p>previous result</p>"}`
	for _, kind := range []string{"webview", "remote", "file"} {
		t.Run(kind, func(t *testing.T) {
			preview := queryPreview{PreviewType: kind, PreviewData: html}
			app := bindTestApp(&App{
				visible:      true,
				querySession: queryController{query: plainQuery{QueryID: "new-query", QueryText: "random ip s"}},
				resultList: resultController{
					resultsQueryID: "old-query",
					selected:       0,
				},
				webView: webViewPreviewController{webViewPreviewData: html},
			})
			if kind == "remote" {
				preview.PreviewData = "/preview?id=old"
				app.remotePreview.remotePreviews = map[string]queryPreview{preview.PreviewData: {PreviewType: "webview", PreviewData: html}}
			} else if kind == "file" {
				preview.PreviewData = "preview.html"
				app.filePreview.filePreviews = map[string]filePreviewContent{preview.PreviewData: {Kind: "webview", WebViewData: html}}
			}
			app.resultList.results = []queryResult{{Preview: preview}}
			app.querySession.beginQueryTransitionLocked(true)
			if app.querySession.queryTransitionTimer == nil {
				t.Fatal("result grace period was not scheduled")
			}
			// Drive the deadline explicitly without posting to a native UI event loop.
			app.querySession.queryTransitionTimer.Stop()
			defer app.querySession.resetQueryTransitionLocked()
			if _, _, visible := app.previews.selectedPreviewForLifecycle(); !visible {
				t.Fatal("active WebView was hidden before retained results expired")
			}
			app.previews.reconcileSelectedPreviewOnUI()
			if app.webView.webViewPreviewData != html {
				t.Fatal("query transition deactivated the retained WebView")
			}
			app.querySession.resetQueryTransitionLocked()
			app.previews.reconcileSelectedPreviewOnUI()
			if app.webView.webViewPreviewData != "" {
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
			app := bindTestApp(&App{
				visible:      true,
				querySession: queryController{query: plainQuery{QueryID: "new-query", QueryText: "random ip s"}},
				resultList: resultController{
					resultsQueryID: "old-query",
					selected:       0,
					results:        []queryResult{{Preview: queryPreview{PreviewType: "webview", PreviewData: html}}},
				},
				webView: webViewPreviewController{webViewPreviewData: html},
			})
			app.querySession.beginQueryTransitionLocked(true)
			app.querySession.queryTransitionTimer.Stop()
			defer app.querySession.resetQueryTransitionLocked()
			switch change {
			case "hidden":
				app.visible = false
			case "destroyed":
				app.destroyed.Store(true)
			case "different-preview":
				app.resultList.results[0].Preview.PreviewData = `{"html":"<p>different</p>"}`
			case "error":
				app.webView.webViewPreviewError = "load failed"
			case "empty-results":
				app.resultList.results = nil
			case "new-results":
				app.resultList.resultsQueryID = app.querySession.query.QueryID
				app.resultList.results[0].Preview = queryPreview{PreviewType: "text", PreviewData: "new result"}
			case "no-preview-layout":
				ratio := float64(1)
				app.resultList.layout.ResultPreviewWidthRatio = &ratio
			}
			app.previews.reconcileSelectedPreviewOnUI()
			if app.webView.webViewPreviewData != "" {
				t.Fatal("unrendered WebView survived reconciliation")
			}
		})
	}
}

func TestNativeFilePreviewLifecycleAdvancesGeneration(t *testing.T) {
	app := bindTestApp(&App{})
	changed := app.nativeFile.activateNativeFilePreview("first.docx")
	if !changed || app.nativeFile.nativeFilePreviewGeneration != 1 {
		t.Fatalf("first native preview activation = changed %v generation %d", changed, app.nativeFile.nativeFilePreviewGeneration)
	}

	app.nativeFile.deactivateNativeFilePreview()
	if app.nativeFile.nativeFilePreviewGeneration != 2 {
		t.Fatalf("native preview deactivation generation = %d, want 2", app.nativeFile.nativeFilePreviewGeneration)
	}
	changed = app.nativeFile.activateNativeFilePreview("second.docx")
	if !changed || app.nativeFile.nativeFilePreviewGeneration != 3 {
		t.Fatalf("second native preview activation = changed %v generation %d, want generation 3", changed, app.nativeFile.nativeFilePreviewGeneration)
	}
}

func TestNativeFilePreviewIgnoresStaleErrors(t *testing.T) {
	app := bindTestApp(&App{
		nativeFile: nativeFilePreviewController{
			nativeFilePreviewPath:       "current.docx",
			nativeFilePreviewGeneration: 2,
		},
	})
	app.nativeFile.setNativeFilePreviewError(1, errors.New("stale preview failure"))
	if app.nativeFile.nativeFilePreviewError != "" || app.nativeFile.nativeFilePreviewGeneration != 2 {
		t.Fatalf("stale native preview error changed state: error %q generation %d", app.nativeFile.nativeFilePreviewError, app.nativeFile.nativeFilePreviewGeneration)
	}
}

func TestNativeFilePreviewDelayedActivationRejectsObsoleteSelection(t *testing.T) {
	app := bindTestApp(&App{})
	if !app.nativeFile.scheduleNativeFilePreview("first.docx") {
		t.Fatal("initial native preview should schedule")
	}
	generation := app.nativeFile.nativeFilePreviewGeneration
	if app.nativeFile.nativeFilePreviewPendingPath != "first.docx" || app.nativeFile.nativeFilePreviewTimer == nil {
		t.Fatal("scheduled native preview did not retain its cancellation state")
	}

	app.nativeFile.deactivateNativeFilePreview()
	app.nativeFile.activateScheduledNativeFilePreview("first.docx", generation)
	if app.nativeFile.nativeFilePreviewPath != "" {
		t.Fatalf("obsolete delayed preview became active for %q", app.nativeFile.nativeFilePreviewPath)
	}
}

func TestNativeFilePreviewCoalescesPendingBounds(t *testing.T) {
	app := bindTestApp(&App{
		nativeFile: nativeFilePreviewController{
			nativeFilePreviewPath:       "document.docx",
			nativeFilePreviewGeneration: 3,
		},
	})
	first := woxui.Rect{X: 1, Y: 2, Width: 300, Height: 400}
	latest := woxui.Rect{X: 2, Y: 3, Width: 320, Height: 420}
	app.nativeFile.requestNativeFilePreviewBounds("document.docx", 3, first)
	timer := app.nativeFile.nativeFilePreviewBoundsTimer
	if timer == nil {
		t.Fatal("first native preview bounds should schedule one deferred update")
	}
	app.nativeFile.requestNativeFilePreviewBounds("document.docx", 3, latest)
	if app.nativeFile.nativeFilePreviewBoundsTimer != timer {
		t.Fatal("new bounds should update the pending request instead of scheduling another native operation")
	}
	if app.nativeFile.nativeFilePreviewBounds != latest {
		t.Fatalf("pending native preview bounds = %+v, want %+v", app.nativeFile.nativeFilePreviewBounds, latest)
	}
	app.nativeFile.stopNativeFilePreviewTimers()
}

// TestPreviewPinFollowsSelectedSurface covers transitions that never build another image body.
func TestPreviewPinFollowsSelectedSurface(t *testing.T) {
	for _, transition := range []string{"same-image", "hidden", "empty-results", "no-preview", "text", "different-image", "no-preview-layout"} {
		t.Run(transition, func(t *testing.T) {
			source := woxImage{ImageType: "absolute", ImageData: "/tmp/preview.png"}
			app := bindTestApp(&App{
				visible:      true,
				querySession: queryController{query: plainQuery{QueryID: "q"}},
				resultList: resultController{
					resultsQueryID: "q",
					selected:       0,
					results:        []queryResult{{Preview: queryPreview{PreviewType: "image", PreviewData: "absolute:/tmp/preview.png"}}},
				},
				imageStore: imageStore{
					images:        map[string]*woxui.Image{},
					imageLastUsed: map[string]uint64{},
					imageViewport: map[string]struct{}{"preview": {}},
				},
				previews: previewController{pinnedPreview: viewportPreviewPin{source: source, size: 2048, key: "preview"}},
			})
			app.imageStore.insertImageLocked("preview", &woxui.Image{Width: 4096, Height: 4096})
			switch transition {
			case "hidden":
				app.visible = false
			case "empty-results":
				app.resultList.results = nil
			case "no-preview":
				app.resultList.results[0].Preview = queryPreview{}
			case "text":
				app.resultList.results[0].Preview = queryPreview{PreviewType: "text", PreviewData: "next"}
			case "different-image":
				app.resultList.results[0].Preview.PreviewData = "absolute:/tmp/next.png"
			case "no-preview-layout":
				ratio := float64(1)
				app.resultList.layout.ResultPreviewWidthRatio = &ratio
			}
			app.previews.reconcileSelectedPreviewOnUI()
			if transition == "same-image" {
				if app.imageStore.images["preview"] == nil || app.previews.pinnedPreview.key != "preview" {
					t.Fatal("unchanged preview lost its pin")
				}
			} else if len(app.imageStore.imageViewport) != 0 || app.imageStore.images["preview"] != nil || app.previews.pinnedPreview.size != 0 {
				t.Fatal("inactive preview retained its pin or oversized cache entry")
			}
		})
	}
}

func TestInlineWebViewScriptTargetResolvesRemoteEnvelope(t *testing.T) {
	const page = `{"html":"<p>initial state</p>"}`
	for _, kind := range []string{"webview", "remote", "unresolved", "url", "hidden", "stale"} {
		t.Run(kind, func(t *testing.T) {
			preview := queryPreview{PreviewType: "webview", PreviewData: page}
			app := bindTestApp(&App{
				sessionID:    "session",
				querySession: queryController{query: plainQuery{QueryID: "query"}},
				resultList: resultController{
					resultsQueryID: "query",
					selected:       0,
					results:        []queryResult{{ID: "row"}},
				},
			})
			if kind == "remote" || kind == "unresolved" {
				preview = queryPreview{PreviewType: "remote", PreviewData: "/preview?id=row"}
				if kind == "remote" {
					app.remotePreview.remotePreviews = map[string]queryPreview{preview.PreviewData: {PreviewType: "webview", PreviewData: page}}
				}
			}
			if kind == "url" {
				preview.PreviewData = `{"url":"https://example.com"}`
			}
			if kind == "stale" {
				app.querySession.query.QueryID = "next"
			}
			for revision := 0; revision < 3; revision++ {
				app.resultList.results[0].Revision++
				key, html := app.previews.inlineWebViewScriptTarget(preview, kind != "hidden")
				if kind == "webview" || kind == "remote" {
					if key != app.webView.webViewDocumentKey() || html != "<p>initial state</p>" {
						t.Fatalf("lost target: %q %q", key, html)
					}
				} else if key != "" || html != "" {
					t.Fatalf("retained invalid target: %q", key)
				}
			}
		})
	}
}

func TestRestoreInlinePreviewRequiresFreshSnapshot(t *testing.T) {
	app := bindTestApp(&App{
		querySession: queryController{query: plainQuery{QueryID: "q"}},
		resultList: resultController{
			resultsQueryID: "q",
			selected:       0,
			results:        []queryResult{{ID: "row", Preview: queryPreview{PreviewType: "remote", PreviewData: "/preview"}}},
		},
		remotePreview: remotePreviewController{remotePreviews: map[string]queryPreview{"/preview": {PreviewType: "webview", PreviewData: `{"html":"<p>old progress</p>"}`}}},
	})
	if !app.previews.hasInlinePreviewToRestore() {
		t.Fatal("hidden inline document must refresh even if task stopped publishing")
	}
	app.remotePreview.remotePreviews["/preview"] = queryPreview{PreviewType: "webview", PreviewData: `{"url":"https://example.com"}`}
	if app.previews.hasInlinePreviewToRestore() {
		t.Fatal("retained URL page must not refresh the query")
	}
	app.resultList.selected = -1
	if app.previews.hasInlinePreviewToRestore() {
		t.Fatal("missing selection must not refresh")
	}
}
