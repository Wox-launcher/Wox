//go:build wox_ui_smoke

package query

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/clipboard"
)

const queryClipboardSample = "smoke-query-clipboard"

// Test004LauncherQueryClipboardEdit verifies Select All, Copy, Cut, and Paste through query-field accessibility actions.
// Flow: show launcher -> set query text -> Select All -> Copy -> Cut -> Paste.
// Evidence: selection spans the full query, the system clipboard receives the text, Cut clears the query, and Paste restores it.
func Test004LauncherQueryClipboardEdit(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.PreserveClipboard(t)
		smoke.ShowLauncher(t, ctx, client)

		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, queryClipboardSample); err != nil {
			t.Fatalf("set query text: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found && input.Value == queryClipboardSample
		}); err != nil {
			t.Fatalf("wait for query text: %v", err)
		}

		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSelectAll, ""); err != nil {
			t.Fatalf("select all query text: %v", err)
		}
		runeCount := len([]rune(queryClipboardSample))
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found && input.Value == queryClipboardSample && input.HasTextSelection &&
				input.SelectionStart == 0 && input.SelectionEnd == runeCount
		}); err != nil {
			t.Fatalf("wait for full query selection: %v", err)
		}

		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionCopy, ""); err != nil {
			t.Fatalf("copy selected query text: %v", err)
		}
		if text, err := clipboard.ReadText(); err != nil || text != queryClipboardSample {
			t.Fatalf("clipboard after copy = %q err %v, want %q", text, err, queryClipboardSample)
		}

		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionCut, ""); err != nil {
			t.Fatalf("cut selected query text: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found && input.Value == ""
		}); err != nil {
			t.Fatalf("wait for empty query after cut: %v", err)
		}
		if text, err := clipboard.ReadText(); err != nil || text != queryClipboardSample {
			t.Fatalf("clipboard after cut = %q err %v, want %q", text, err, queryClipboardSample)
		}

		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionPaste, ""); err != nil {
			t.Fatalf("paste query text: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found && input.Value == queryClipboardSample
		})
		if err != nil {
			t.Fatalf("wait for restored query after paste: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
