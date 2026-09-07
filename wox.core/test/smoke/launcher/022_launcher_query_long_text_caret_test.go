//go:build wox_ui_smoke

package query

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/clipboard"
)

const queryLongTextTail = "lkjadfij lk jlkajdflkja fkjasf;l sajkf sakl;f jsd;l fjslda;kf jsdl;jf klsdjfklsdjf overflow-caret-smoke"

// Test022LauncherQueryLongTextCaret verifies a long single-line query keeps the caret inside the input.
// Flow: show launcher -> type a prefix -> paste overflowing text -> Home -> End.
// Evidence: the query IME caret stays inside the focused input bounds after paste and after jumping to each end.
func Test022LauncherQueryLongTextCaret(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.PreserveClipboard(t)
		smoke.ShowLauncher(t, ctx, client)

		const prefix = "wox-smoke-long-query "
		if err := client.EnterText(ctx, prefix); err != nil {
			t.Fatalf("type long-query prefix: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			return queryCaretVisible(snapshot, prefix)
		}); err != nil {
			t.Fatalf("wait for typed prefix caret: %v", err)
		}

		if err := clipboard.WriteText(queryLongTextTail); err != nil {
			t.Fatalf("prepare overflowing clipboard text: %v", err)
		}
		modifier := woxui.KeyModifierControl
		if runtime.GOOS == "darwin" {
			modifier = woxui.KeyModifierMeta
		}
		handled, err := client.PressKeyHandled(ctx, woxui.Key("v"), modifier)
		if err != nil {
			t.Fatalf("paste overflowing query text: %v", err)
		}
		if !handled {
			t.Fatal("paste overflowing query text: key was not handled")
		}
		query := prefix + queryLongTextTail
		end := len([]rune(query))
		snapshot := waitQueryCaret(t, ctx, client, query, end, end)

		if err := client.PressKey(ctx, woxui.KeyHome, 0); err != nil {
			t.Fatalf("move caret to the start of the long query: %v", err)
		}
		waitQueryCaret(t, ctx, client, query, 0, 0)

		if err := client.PressKey(ctx, woxui.KeyEnd, 0); err != nil {
			t.Fatalf("move caret to the end of the long query: %v", err)
		}
		waitQueryCaret(t, ctx, client, query, end, end)
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

func waitQueryCaret(t *testing.T, ctx context.Context, client *automationdriver.Client, query string, start, end int) woxwidget.AutomationSnapshot {
	t.Helper()
	snapshot, err := client.WaitForReason(ctx, func(snapshot woxwidget.AutomationSnapshot) (bool, string) {
		input, found := automationdriver.Find(snapshot, "launcher.query.input")
		if !found {
			return false, "query input missing"
		}
		if input.Value != query || !input.HasTextSelection || input.SelectionStart != start || input.SelectionEnd != end {
			return false, fmt.Sprintf("value=%q selection=%d:%d", input.Value, input.SelectionStart, input.SelectionEnd)
		}
		if !queryCaretInBounds(input) {
			return false, fmt.Sprintf("caret=%v bounds=%v", input.CursorRect, input.Bounds)
		}
		return true, ""
	})
	if err != nil {
		t.Fatalf("wait for visible caret in %q at %d:%d: %v", query, start, end, err)
	}
	return snapshot
}

func queryCaretVisible(snapshot woxwidget.AutomationSnapshot, query string) bool {
	input, found := automationdriver.Find(snapshot, "launcher.query.input")
	return found && input.Value == query && input.Focused && queryCaretInBounds(input)
}

func queryCaretInBounds(input woxui.AccessibilityNode) bool {
	caret := input.CursorRect
	if caret.Width <= 0 || caret.Height <= 0 {
		return false
	}
	return caret.X >= input.Bounds.X && caret.Y >= input.Bounds.Y &&
		caret.X+caret.Width <= input.Bounds.X+input.Bounds.Width &&
		caret.Y+caret.Height <= input.Bounds.Y+input.Bounds.Height
}
