//go:build wox_ui_smoke

package timer

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test001LauncherTimerDesktopOverlay verifies the pinned Timer uses a real interactive desktop overlay.
// Flow: start a pinned Timer -> inspect and hover its overlay -> close it -> pin it again -> delete the Timer.
// Evidence: the native overlay exposes a live countdown and note, closes through its own button, reopens from the Timer action, and disappears on deletion.
func Test001LauncherTimerDesktopOverlay(t *testing.T) {
	const note = "wox smoke desktop overlay"

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		cleanupTimer(t, client, note)
		smoke.ShowLauncher(t, ctx, client)
		timerID := startPinnedTimer(t, ctx, client, "2m", note)
		overlayInstance := timerOverlayInstancePrefix + timerID

		if _, err := client.WaitForWindowState(ctx, overlayInstance, func(state automationdriver.WindowState) bool {
			return state.Exists && state.Visible && state.Lifecycle == "visible"
		}); err != nil {
			t.Fatalf("wait for Timer desktop overlay: %v", err)
		}
		if err := client.FocusInstance(ctx, overlayInstance); err != nil {
			t.Fatalf("focus Timer desktop overlay: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			countdown, found := automationdriver.Find(snapshot, "timer-overlay-countdown")
			seconds, valid := countdownSeconds(countdown.Value)
			_, noteFound := automationdriver.Find(snapshot, "timer-overlay-note")
			_, closeFound := automationdriver.Find(snapshot, "timer-overlay-close")
			return found && valid && seconds >= 100 && seconds <= 120 && !noteFound && !closeFound
		})
		if err != nil {
			t.Fatalf("wait for compact Timer overlay countdown: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)

		if err := client.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerEnter}); err != nil {
			t.Fatalf("hover Timer desktop overlay: %v", err)
		}
		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			noteNode, noteFound := automationdriver.Find(snapshot, "timer-overlay-note")
			closeNode, closeFound := automationdriver.Find(snapshot, "timer-overlay-close")
			return noteFound && noteNode.Value == note && closeFound && closeNode.Role == woxui.AccessibilityRoleButton
		})
		if err != nil {
			t.Fatalf("wait for expanded Timer overlay controls: %v", err)
		}
		if err := client.Perform(ctx, "timer-overlay-close", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("close Timer desktop overlay: %v", err)
		}
		if _, err := client.WaitForWindowState(ctx, overlayInstance, func(state automationdriver.WindowState) bool {
			return !state.Exists
		}); err != nil {
			t.Fatalf("wait for Timer desktop overlay to close: %v", err)
		}

		if err := client.FocusInstance(ctx, "primary"); err != nil {
			t.Fatalf("focus primary Launcher after closing Timer overlay: %v", err)
		}
		smoke.ShowLauncher(t, ctx, client)
		selectTimerByNote(t, ctx, client, note)
		smoke.ActivateSelectedResultAction(t, ctx, client, "action-result-"+timerID+":pin-")
		if _, err := client.WaitForWindowState(ctx, overlayInstance, func(state automationdriver.WindowState) bool {
			return state.Exists && state.Visible
		}); err != nil {
			t.Fatalf("wait for Timer desktop overlay to reopen: %v", err)
		}

		deleteTimer(t, ctx, client, note)
		if _, err := client.WaitForWindowState(ctx, overlayInstance, func(state automationdriver.WindowState) bool {
			return !state.Exists
		}); err != nil {
			t.Fatalf("wait for Timer desktop overlay to close after deletion: %v", err)
		}
	})
}
