//go:build wox_ui_smoke

package websearch

import (
	"context"
	"testing"
	"time"

	"wox/plugin"
	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test009LauncherWebsearchWebviewBackground verifies Run in background changes whether hiding Wox reloads the page.
// Flow: save a default WebView search -> query, hide, and show -> enable Run in background -> query again, hide, and show.
// Evidence: the local document is requested again when the option is off, and the same preview returns with no new request when it is on.
func Test009LauncherWebsearchWebviewBackground(t *testing.T) {
	skipWebSearchWebViewIfUnavailable(t)

	const keyword = "woxsmokewvb"
	token := plugin.ParameterQueryVariable("query")
	title := "Background Preview " + token
	query := keyword + " hello"

	server, requests := startCountingWebSearchPreviewServer(t)
	urls := server.URL + "/search?q=" + token
	previewURL := server.URL + "/search?q=hello"

	smoke.CaseWithTimeout(t, 120*time.Second, func(ctx context.Context, client *automationdriver.Client) {
		t.Cleanup(func() { deleteWebSearchRow(t, client, keyword) })
		addWebSearchAutomaticPreviewRow(t, ctx, client, keyword, title, urls, "", "", false)
		confirmWebSearchAutomaticPreviewPersisted(t, ctx, client, keyword, title, urls, "", "", false)

		queryWebSearchAutomaticPreview(t, ctx, client, keyword, query, previewURL, requests)
		reloaded := requests.Load()
		hideAndShowWebSearchLauncher(t, ctx, client)
		waitForWebSearchAutomaticPreview(t, ctx, client, query, previewURL, requests, reloaded+1)

		setWebSearchKeepInBackground(t, ctx, client, keyword, true)
		confirmWebSearchAutomaticPreviewPersisted(t, ctx, client, keyword, title, urls, "", "", true)
		queryWebSearchAutomaticPreview(t, ctx, client, keyword, query, previewURL, requests)
		retained := requests.Load()
		hideAndShowWebSearchLauncher(t, ctx, client)
		waitForWebSearchAutomaticPreview(t, ctx, client, query, previewURL, requests, retained)
		assertWebSearchPreviewRetained(t, ctx, client, query, previewURL, requests, retained)
	})
}
