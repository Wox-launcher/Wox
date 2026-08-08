//go:build wox_ui_smoke

package query

import (
	"context"
	"errors"
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
// Flow: show launcher -> insert a line with Shift+Enter -> paste mixed newline text with the platform shortcut -> scroll the overflowing query upward.
// Evidence: the query semantics preserve normalized line breaks and move vertically after the wheel event without changing their value.
func Test001LauncherQueryMultiline(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		previousClipboard, err := clipboard.Read()
		if err != nil && !errors.Is(err, clipboard.NoDataErr()) {
			t.Fatalf("read clipboard before multiline query case: %v", err)
		}
		t.Cleanup(func() {
			if previousClipboard != nil {
				if restoreErr := clipboard.Write(previousClipboard); restoreErr != nil {
					t.Errorf("restore clipboard after multiline query case: %v", restoreErr)
				}
				return
			}
			if restoreErr := clipboard.WriteText(""); restoreErr != nil {
				t.Errorf("clear clipboard after multiline query case: %v", restoreErr)
			}
		})

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

		if err := clipboard.WriteText("\r\nthree\r\nfour\rfive\nsix"); err != nil {
			t.Fatalf("prepare multiline clipboard text: %v", err)
		}
		modifier := woxui.KeyModifierControl
		if runtime.GOOS == "darwin" {
			modifier = woxui.KeyModifierMeta
		}
		if err := client.PressKey(ctx, woxui.Key("v"), modifier); err != nil {
			t.Fatalf("paste multiline query text: %v", err)
		}
		expected := "one\ntwo\nthree\nfour\nfive\nsix"
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found && input.Value == expected
		})
		if err != nil {
			t.Fatalf("wait for pasted multiline query: %v", err)
		}
		before, found := automationdriver.Find(snapshot, "launcher.query.input")
		if !found {
			t.Fatal("launcher query input disappeared before wheel scroll")
		}
		beforeScroll, found := automationdriver.Find(snapshot, "launcher.query.scroll")
		if !found {
			t.Fatal("launcher query scroll state is unavailable")
		}
		if err := client.Pointer(ctx, woxui.PointerEvent{
			Kind:     woxui.PointerScroll,
			Position: woxui.Point{X: before.Bounds.X + 20, Y: before.Bounds.Y + before.Bounds.Height/2},
			Scroll:   woxui.Point{Y: -34},
		}); err != nil {
			t.Fatalf("scroll multiline query with wheel: %v", err)
		}
		scrollCtx, cancelScroll := context.WithTimeout(ctx, 5*time.Second)
		defer cancelScroll()
		snapshot, err = client.WaitFor(scrollCtx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			scroll, scrollFound := automationdriver.Find(snapshot, "launcher.query.scroll")
			return found && input.Value == expected && scrollFound && scroll.Value != beforeScroll.Value
		})
		if err != nil {
			currentSnapshot, snapshotErr := client.Snapshot(ctx)
			current, currentFound := automationdriver.Find(currentSnapshot, "launcher.query.scroll")
			t.Fatalf("wait for multiline query wheel scroll: %v; before %q current found %v value %q snapshot error %v", err, beforeScroll.Value, currentFound, current.Value, snapshotErr)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
