//go:build wox_ui_smoke

package websearch

import (
	"context"
	"testing"

	"wox/plugin"
	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test008LauncherWebsearchWebviewDefaultPreview verifies Open in WebView shows the configured page and window immediately.
// Flow: save a local search with default WebView, width, and height -> query it -> press Enter.
// Evidence: the preview URL and native window size appear without the preview hotkey, and Enter focuses the same page without reloading it.
func Test008LauncherWebsearchWebviewDefaultPreview(t *testing.T) {
	skipWebSearchWebViewIfUnavailable(t)

	const (
		keyword       = "woxsmokewvd"
		previewWidth  = float32(640)
		previewHeight = float32(520)
	)
	token := plugin.ParameterQueryVariable("query")
	title := "Default Preview " + token
	query := keyword + " hello"
	widthText := "640"
	heightText := "520"

	server, requests := startCountingWebSearchPreviewServer(t)
	urls := server.URL + "/search?q=" + token
	previewURL := server.URL + "/search?q=hello"

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		t.Cleanup(func() { deleteWebSearchRow(t, client, keyword) })
		addWebSearchAutomaticPreviewRow(t, ctx, client, keyword, title, urls, widthText, heightText, false)
		confirmWebSearchAutomaticPreviewPersisted(t, ctx, client, keyword, title, urls, widthText, heightText, false)

		queryWebSearchAutomaticPreview(t, ctx, client, keyword, query, previewURL, requests)
		waitForWebSearchPreviewBounds(t, ctx, client, previewWidth, previewHeight)
		focusWebSearchAutomaticPreview(t, ctx, client, query, previewURL, requests)
	})
}
