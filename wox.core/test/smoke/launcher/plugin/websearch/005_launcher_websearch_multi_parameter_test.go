//go:build wox_ui_smoke

package websearch

import (
	"context"
	"testing"

	"wox/plugin"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
)

// Test005LauncherWebsearchMultiParameter verifies two URL parameters become Tab-separated query slots.
// Flow: save a two-parameter Web Search -> type the first value -> Tab -> type the second value.
// Evidence: the hint shows both names, then the result title contains both typed values.
func Test005LauncherWebsearchMultiParameter(t *testing.T) {
	const keyword = "woxsmokemp"
	textToken := plugin.ParameterQueryVariable("text")
	langToken := plugin.ParameterQueryVariable("lang")
	title := textToken + " / " + langToken
	urls := "https://example.com/?t=" + textToken + "&l=" + langToken

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		t.Cleanup(func() { deleteWebSearchRow(t, client, keyword) })
		addWebSearchRow(t, ctx, client, keyword, title, urls)
		confirmWebSearchRowPersisted(t, ctx, client, keyword, title, urls)

		typeWebSearchQuery(t, ctx, client, keyword)
		waitForWebSearchHint(t, ctx, client, keyword+" ", "text", "lang")
		if err := client.EnterText(ctx, "hello"); err != nil {
			t.Fatal(err)
		}
		if err := client.PressKey(ctx, woxui.KeyTab, 0); err != nil {
			t.Fatal(err)
		}
		if err := client.EnterText(ctx, "en"); err != nil {
			t.Fatal(err)
		}
		waitForWebSearchResult(t, ctx, client, keyword+" hello en", "hello / en")
	})
}
