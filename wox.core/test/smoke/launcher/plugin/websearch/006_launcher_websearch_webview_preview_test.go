//go:build wox_ui_smoke

package websearch

import (
	"context"
	"testing"

	"wox/plugin"
	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test006LauncherWebsearchWebviewPreview verifies Ctrl/Cmd+Enter opens an in-launcher WebView and Escape returns to the search.
// Flow: save a local-search Web Search -> query it -> press the primary+Enter preview hotkey -> press Escape.
// Evidence: the preview node shows the local URL while the query box stays, then the result list returns after Escape.
func Test006LauncherWebsearchWebviewPreview(t *testing.T) {
	skipWebSearchWebViewIfUnavailable(t)

	const keyword = "woxsmokewv"
	token := plugin.ParameterQueryVariable("query")
	title := "Preview " + token
	query := keyword + " hello"
	resultTitle := "Preview hello"

	server := startWebSearchPreviewServer(t)
	urls := server.URL + "/search?q=" + token
	previewURL := server.URL + "/search?q=hello"

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		t.Cleanup(func() { deleteWebSearchRow(t, client, keyword) })
		addWebSearchRow(t, ctx, client, keyword, title, urls)
		confirmWebSearchRowPersisted(t, ctx, client, keyword, title, urls)

		typeWebSearchQuery(t, ctx, client, keyword)
		waitForWebSearchHint(t, ctx, client, keyword+" ", "query")
		if err := client.EnterText(ctx, "hello"); err != nil {
			t.Fatal(err)
		}
		waitForWebSearchResult(t, ctx, client, query, resultTitle)

		openWebSearchWebViewPreview(t, ctx, client, query, previewURL)
		exitWebSearchWebViewPreview(t, ctx, client, query, resultTitle)
	})
}
