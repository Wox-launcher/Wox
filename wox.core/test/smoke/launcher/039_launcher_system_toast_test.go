//go:build wox_ui_smoke

package query

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test039LauncherSystemToastKeepsToolbar verifies a system toast is independent of existing toolbar state.
// Flow: show a toolbar message -> open a system toast -> activate its action.
// Evidence: both messages remain visible together and the toast action opens its target query.
func Test039LauncherSystemToastKeepsToolbar(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, "wox-smoke system-toast ")
		id, found := smoke.FindLauncherResult(snapshot, "Show system toast fixture")
		if !found {
			t.Fatal("system toast fixture not found")
		}
		if err := client.Perform(ctx, id, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitForWindowState(ctx, "overlay.wox_notifier", func(state automationdriver.WindowState) bool { return state.Exists && state.Visible }); err != nil {
			t.Fatal(err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return found && status.Value == "Toolbar fixture ready"
		})
		if err != nil {
			t.Fatalf("toolbar message replaced by toast: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		if err := client.FocusInstance(ctx, "overlay.wox_notifier"); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			message, found := automationdriver.Find(snapshot, "text-overlay-message-wox_notifier")
			return found && message.Value == "System toast fixture"
		}); err != nil {
			t.Fatalf("system toast message: %v", err)
		}
		if err := client.Perform(ctx, "toast-action", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		if err := client.FocusInstance(ctx, "primary"); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := smoke.FindLauncherResult(snapshot, "Toast action completed")
			return found
		}); err != nil {
			t.Fatalf("toast action result: %v", err)
		}
	})
}
