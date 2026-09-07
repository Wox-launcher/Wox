//go:build wox_ui_smoke

package websearch

import (
	"context"
	"testing"

	"wox/plugin"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test004LauncherWebsearchTitleValidation verifies saving is blocked when the title names an undeclared parameter.
// Flow: add a Web Search row -> put a parameter only in the title -> save.
// Evidence: the title field shows a validation error and the row editor stays open.
func Test004LauncherWebsearchTitleValidation(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		openWebSearchSettings(t, ctx, client)
		if err := client.Perform(ctx, webSearchesTableAddID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("add Web Search row: %v", err)
		}
		waitForWebSearchRowEditor(t, ctx, client)
		setWebSearchRowText(t, ctx, client, webSearchKeywordFieldID, "woxsmokebad")
		setWebSearchRowText(t, ctx, client, webSearchTitleFieldID, "Search "+plugin.ParameterQueryVariable("missing"))
		setWebSearchRowText(t, ctx, client, webSearchUrlsFieldID, "https://example.com/")
		enableWebSearchRow(t, ctx, client)
		saveWebSearchRow(t, ctx, client)
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			errorNode, errorFound := automationdriver.Find(snapshot, webSearchTitleErrorID)
			_, saveFound := automationdriver.Find(snapshot, "form-table-row-save")
			return errorFound && errorNode.Value != "" && saveFound
		}); err != nil {
			t.Fatalf("wait for undeclared title parameter error: %v", err)
		}
		if err := client.Perform(ctx, "form-table-row-cancel", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("cancel invalid Web Search row: %v", err)
		}
	})
}
