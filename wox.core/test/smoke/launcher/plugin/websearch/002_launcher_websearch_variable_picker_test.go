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

// Test002LauncherWebsearchVariablePicker verifies inserting an input parameter from the {} picker.
// Flow: add a Web Search row -> insert a parameter into the URL -> save -> query the keyword.
// Evidence: the persisted URL contains the parameter token and the launcher result uses the typed value.
func Test002LauncherWebsearchVariablePicker(t *testing.T) {
	const keyword = "woxsmokeqv"
	title := "Smoke Search " + plugin.ParameterQueryVariable("query")
	urlPrefix := "https://example.com/search?q="
	urls := urlPrefix + plugin.ParameterQueryVariable("query")

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		t.Cleanup(func() { deleteWebSearchRow(t, client, keyword) })
		openWebSearchSettings(t, ctx, client)
		if err := client.Perform(ctx, webSearchesTableAddID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("add Web Search row: %v", err)
		}
		waitForWebSearchRowEditor(t, ctx, client)
		setWebSearchRowText(t, ctx, client, webSearchKeywordFieldID, keyword)
		setWebSearchRowText(t, ctx, client, webSearchTitleFieldID, title)
		setWebSearchRowText(t, ctx, client, webSearchUrlsFieldID, urlPrefix)
		insertWebSearchParameterFromPicker(t, ctx, client)
		enableWebSearchRow(t, ctx, client)
		saveWebSearchRow(t, ctx, client)
		waitForWebSearchEditorClosed(t, ctx, client)
		confirmWebSearchRowPersisted(t, ctx, client, keyword, title, urls)

		typeWebSearchQuery(t, ctx, client, keyword)
		waitForWebSearchHint(t, ctx, client, keyword+" ", "query")
		if err := client.EnterText(ctx, "hello"); err != nil {
			t.Fatal(err)
		}
		waitForWebSearchResult(t, ctx, client, keyword+" hello", "Smoke Search hello")
	})
}
