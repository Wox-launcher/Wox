//go:build wox_ui_smoke

package websearch

import (
	"context"
	"math"
	"testing"

	"wox/plugin"
	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test007LauncherWebsearchWebviewPreviewSize verifies configured preview width and height size the launcher window.
// Flow: save a local-search Web Search with WebView width and height -> query it -> open the in-launcher preview.
// Evidence: native launcher bounds match those logical pixels and differ from the pre-preview result window.
func Test007LauncherWebsearchWebviewPreviewSize(t *testing.T) {
	skipWebSearchWebViewIfUnavailable(t)

	const (
		keyword       = "woxsmokewvs"
		previewWidth  = float32(640)
		previewHeight = float32(520)
	)
	token := plugin.ParameterQueryVariable("query")
	title := "Sized Preview " + token
	query := keyword + " hello"
	resultTitle := "Sized Preview hello"
	widthText := "640"
	heightText := "520"

	server := startWebSearchPreviewServer(t)
	urls := server.URL + "/search?q=" + token
	previewURL := server.URL + "/search?q=hello"

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		t.Cleanup(func() { deleteWebSearchRow(t, client, keyword) })
		addWebSearchRowWithPreviewSize(t, ctx, client, keyword, title, urls, widthText, heightText)
		confirmWebSearchRowPreviewSizePersisted(t, ctx, client, keyword, title, urls, widthText, heightText)

		typeWebSearchQuery(t, ctx, client, keyword)
		waitForWebSearchHint(t, ctx, client, keyword+" ", "query")
		if err := client.EnterText(ctx, "hello"); err != nil {
			t.Fatal(err)
		}
		waitForWebSearchResult(t, ctx, client, query, resultTitle)
		before := readLauncherBounds(t, ctx, client)

		openWebSearchWebViewPreview(t, ctx, client, query, previewURL)
		waitForWebSearchPreviewBounds(t, ctx, client, previewWidth, previewHeight)
		after := readLauncherBounds(t, ctx, client)
		if math.Abs(float64(after.Width-before.Width)) <= 1 && math.Abs(float64(after.Height-before.Height)) <= 1 {
			t.Fatalf("preview size did not change the launcher window: before %+v after %+v", before, after)
		}
		exitWebSearchWebViewPreview(t, ctx, client, query, resultTitle)
	})
}
