//go:build wox_ui_smoke

package query

import (
	"context"
	"runtime"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/clipboard"
)

// Test038LauncherToolbarLongMessage verifies a truncated message without plugin actions remains readable.
// Flow: show a long toolbar message -> open its automatic actions -> copy it -> open the full-message toast.
// Evidence: the clipboard and toast contain the complete text and the toast remains until dismissed.
func Test038LauncherToolbarLongMessage(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.PreserveClipboard(t)
		smoke.ShowLauncher(t, ctx, client)
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "wox-smoke toolbar-long-plain ")
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return found && status.Value == toolbarLongSmokeStatus && status.Role == woxui.AccessibilityRoleButton
		})
		if err != nil {
			t.Fatalf("wait for truncated message entry: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		modifier := woxui.KeyModifierControl
		if runtime.GOOS == "darwin" {
			modifier = woxui.KeyModifierMeta
		}
		if err := client.PressKey(ctx, woxui.Key("t"), modifier); err != nil {
			t.Fatal(err)
		}
		waitLongMessageActions(t, ctx, client)
		if err := client.Perform(ctx, "action-toolbar-copy-message", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		if text, err := clipboard.ReadText(); err != nil || text != toolbarLongSmokeStatus {
			t.Fatalf("copied text = %q err %v", text, err)
		}
		if err := client.Perform(ctx, "launcher.toolbar.status", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		waitLongMessageActions(t, ctx, client)
		if err := client.Perform(ctx, "action-toolbar-show-message", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitForWindowState(ctx, "overlay.wox_notifier", func(state automationdriver.WindowState) bool { return state.Exists && state.Visible }); err != nil {
			t.Fatalf("wait for full message toast: %v", err)
		}
		if err := client.FocusInstance(ctx, "overlay.wox_notifier"); err != nil {
			t.Fatal(err)
		}
		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			message, found := automationdriver.Find(snapshot, "text-overlay-message-wox_notifier")
			return found && message.Value == toolbarLongSmokeStatus
		})
		if err != nil {
			t.Fatalf("wait for full toast message: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		if err := client.Perform(ctx, "text-overlay-close", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("close full message toast: %v", err)
		}
		if _, err := client.WaitForWindowState(ctx, "overlay.wox_notifier", func(state automationdriver.WindowState) bool { return !state.Exists }); err != nil {
			t.Fatalf("wait for toast dismissal: %v", err)
		}
		if err := client.FocusInstance(ctx, "primary"); err != nil {
			t.Fatal(err)
		}
	})
}

func waitLongMessageActions(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, copy := automationdriver.Find(snapshot, "action-toolbar-copy-message")
		_, show := automationdriver.Find(snapshot, "action-toolbar-show-message")
		return copy && show
	})
	if err != nil {
		t.Fatalf("wait for automatic message actions: %v", err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
}
