//go:build wox_ui_smoke

package query

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/clipboard"
)

// Test001LauncherQueryMultiline verifies multiline keyboard input, clipboard paste, and wheel scrolling in the native query box.
// Flow: show launcher -> insert a line with Shift+Enter -> paste mixed newline text with the platform shortcut -> scroll the overflowing query.
// Evidence: internal line endings are normalized, leading and trailing line endings are dropped, and the query moves vertically after the wheel event without changing its value.
func Test001LauncherQueryMultiline(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.PreserveClipboard(t)

		smoke.ShowLauncher(t, ctx, client)
		if err := client.EnterText(ctx, "one"); err != nil {
			t.Fatalf("enter first query line: %v", err)
		}
		if err := client.PressKey(ctx, woxui.KeyEnter, woxui.KeyModifierShift); err != nil {
			t.Fatalf("insert query newline with Shift+Enter: %v", err)
		}
		if err := client.EnterText(ctx, "two"); err != nil {
			t.Fatalf("enter second query line: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found && input.Value == "one\ntwo"
		}); err != nil {
			t.Fatalf("wait for Shift+Enter multiline query: %v", err)
		}

		if err := clipboard.WriteText("\r\nthree\r\nfour\rfive\nsix\r\n"); err != nil {
			t.Fatalf("prepare multiline clipboard text: %v", err)
		}
		clipboardText, err := clipboard.ReadText()
		if err != nil || clipboardText != "\r\nthree\r\nfour\rfive\nsix\r\n" {
			t.Fatalf("verify multiline clipboard text: value %q err %v", clipboardText, err)
		}
		modifier := woxui.KeyModifierControl
		if runtime.GOOS == "darwin" {
			modifier = woxui.KeyModifierMeta
		}
		handled, err := client.PressKeyHandled(ctx, woxui.Key("v"), modifier)
		if err != nil {
			t.Fatalf("paste multiline query text: %v", err)
		}
		if !handled {
			t.Fatal("paste multiline query text: key was not handled")
		}
		// The pasted text starts and ends with line endings, so "three" continues the current line.
		expected := "one\ntwothree\nfour\nfive\nsix"
		pasteCtx, cancelPaste := context.WithTimeout(ctx, 5*time.Second)
		defer cancelPaste()
		snapshot, err := client.WaitFor(pasteCtx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found && input.Value == expected
		})
		if err != nil {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			t.Fatalf("wait for pasted multiline query: %v; generation %d input found %v value %q diagnostics %v", err, snapshot.Tree.Generation, found, input.Value, snapshot.Diagnostics)
		}
		if _, found := automationdriver.Find(snapshot, "launcher.query.input"); !found {
			t.Fatal("launcher query input disappeared before wheel scroll")
		}
		// The caret may land at either end of the viewport depending on layout timing.
		// Set a known starting position before testing a real wheel event.
		if err := client.Perform(ctx, "launcher.query.scroll", woxui.AccessibilityActionScroll, "0"); err != nil {
			t.Fatalf("reset multiline query scroll: %v", err)
		}
		var beforeScroll woxui.AccessibilityNode
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			var found bool
			beforeScroll, found = automationdriver.Find(snapshot, "launcher.query.scroll")
			var offset, maximum float64
			_, parseErr := fmt.Sscanf(beforeScroll.Value, "%f/%f", &offset, &maximum)
			return inputFound && input.Value == expected && found && parseErr == nil && offset == 0 && maximum > 0
		}); err != nil {
			t.Fatalf("wait for scrollable query at top: %v", err)
		}
		if err := client.Pointer(ctx, woxui.PointerEvent{
			Kind:     woxui.PointerScroll,
			Position: woxui.Point{X: beforeScroll.Bounds.X + 20, Y: beforeScroll.Bounds.Y + beforeScroll.Bounds.Height/2},
			Scroll:   woxui.Point{Y: -34},
		}); err != nil {
			t.Fatalf("scroll multiline query with wheel: %v", err)
		}
		scrollCtx, cancelScroll := context.WithTimeout(ctx, 5*time.Second)
		defer cancelScroll()
		snapshot, err = client.WaitFor(scrollCtx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			scroll, scrollFound := automationdriver.Find(snapshot, "launcher.query.scroll")
			var offset, maximum float64
			_, parseErr := fmt.Sscanf(scroll.Value, "%f/%f", &offset, &maximum)
			return found && input.Value == expected && scrollFound && parseErr == nil && offset > 0
		})
		if err != nil {
			currentSnapshot, snapshotErr := client.Snapshot(ctx)
			current, currentFound := automationdriver.Find(currentSnapshot, "launcher.query.scroll")
			t.Fatalf("wait for multiline query wheel scroll: %v; before %q current found %v value %q snapshot error %v", err, beforeScroll.Value, currentFound, current.Value, snapshotErr)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
