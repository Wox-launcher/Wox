//go:build wox_ui_smoke

package websearch

import (
	"context"
	"testing"

	"wox/plugin"
	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test003LauncherWebsearchNamedParameter verifies a Chinese parameter name becomes the query slot label.
// Flow: save a Web Search whose parameter is named 查询内容 -> type the keyword and space -> type a value.
// Evidence: the hint shows 查询内容 and the result title uses the typed value.
func Test003LauncherWebsearchNamedParameter(t *testing.T) {
	const keyword = "woxsmokezh"
	token := plugin.ParameterQueryVariable("查询内容")
	title := "查找 " + token
	urls := "https://example.com/?q=" + token

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		t.Cleanup(func() { deleteWebSearchRow(t, client, keyword) })
		addWebSearchRow(t, ctx, client, keyword, title, urls)
		confirmWebSearchRowPersisted(t, ctx, client, keyword, title, urls)

		typeWebSearchQuery(t, ctx, client, keyword)
		waitForWebSearchHint(t, ctx, client, keyword+" ", "查询内容")
		if err := client.EnterText(ctx, "你好"); err != nil {
			t.Fatal(err)
		}
		waitForWebSearchResult(t, ctx, client, keyword+" 你好", "查找 你好")
	})
}
