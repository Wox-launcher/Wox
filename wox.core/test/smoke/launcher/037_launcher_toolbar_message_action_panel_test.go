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

// Test037LauncherToolbarMessageActionPanel verifies long plugin messages have their own accessible action panel.
// Flow: show a long message -> inspect result actions -> open message actions by shortcut and click -> copy, update, and clear the message.
// Evidence: panels contain their own actions, Escape closes the panel, the clipboard holds the full message, and plugin callbacks update and clear it.
func Test037LauncherToolbarMessageActionPanel(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.PreserveClipboard(t)
		smoke.ShowLauncher(t, ctx, client)
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, toolbarLongSmokeQuery)
		waitToolbarPanelStatus(t, ctx, client, toolbarLongSmokeStatus, false)

		if err := client.Perform(ctx, "result-toolbar-more", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("open result actions: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, result := automationdriver.FindByAutomationIDPrefix(snapshot, "action-result-hide-launcher-")
			_, toolbar := automationdriver.FindByAutomationIDPrefix(snapshot, "action-toolbar-keep-open-")
			return result && !toolbar
		})
		if err != nil {
			t.Fatalf("wait for separate result panel: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		if err := client.PressKey(ctx, woxui.KeyEscape, 0); err != nil {
			t.Fatalf("close result panel: %v", err)
		}
		waitToolbarPanelStatus(t, ctx, client, toolbarLongSmokeStatus, false)

		modifier := woxui.KeyModifierControl
		if runtime.GOOS == "darwin" {
			modifier = woxui.KeyModifierMeta
		}
		if err := client.PressKey(ctx, woxui.Key("t"), modifier); err != nil {
			t.Fatalf("open message panel by shortcut: %v", err)
		}
		waitToolbarPanelStatus(t, ctx, client, toolbarLongSmokeStatus, true)
		if err := client.PressKey(ctx, woxui.KeyEscape, 0); err != nil {
			t.Fatalf("close message panel with Escape: %v", err)
		}
		waitToolbarPanelStatus(t, ctx, client, toolbarLongSmokeStatus, false)

		if err := client.Perform(ctx, "launcher.toolbar.status", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("open message panel by click: %v", err)
		}
		waitToolbarPanelStatus(t, ctx, client, toolbarLongSmokeStatus, true)
		if err := client.Perform(ctx, "action-toolbar-copy-message", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("copy message: %v", err)
		}
		waitToolbarPanelStatus(t, ctx, client, toolbarLongSmokeStatus, false)
		if text, err := clipboard.ReadText(); err != nil || text != toolbarLongSmokeStatus {
			t.Fatalf("copied message = %q, err %v; want full text %q", text, err, toolbarLongSmokeStatus)
		}

		if err := client.Perform(ctx, "launcher.toolbar.status", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("reopen message panel: %v", err)
		}
		snapshot = waitToolbarPanelStatus(t, ctx, client, toolbarLongSmokeStatus, true)
		keep, _ := automationdriver.FindByAutomationIDPrefix(snapshot, "action-toolbar-keep-open-")
		if err := client.Perform(ctx, keep.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("execute message action: %v", err)
		}
		const updated = "Toolbar fixture keep-open: context-round-trip"
		waitToolbarPanelStatus(t, ctx, client, updated, false)
		if err := client.Perform(ctx, "launcher.toolbar.status", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("open updated message panel: %v", err)
		}
		snapshot = waitToolbarPanelStatus(t, ctx, client, updated, true)
		clear, _ := automationdriver.FindByAutomationIDPrefix(snapshot, "action-toolbar-clear-")
		if err := client.Perform(ctx, clear.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("clear message: %v", err)
		}
		waitForToolbarFixtureGone(t, ctx, client, "wait for message action to clear status")
	})
}

// waitToolbarPanelStatus checks the message and the action panel's visible ownership together.
func waitToolbarPanelStatus(t *testing.T, ctx context.Context, client *automationdriver.Client, message string, open bool) woxwidget.AutomationSnapshot {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
		_, keep := automationdriver.FindByAutomationIDPrefix(snapshot, "action-toolbar-keep-open-")
		_, clear := automationdriver.FindByAutomationIDPrefix(snapshot, "action-toolbar-clear-")
		_, copy := automationdriver.Find(snapshot, "action-toolbar-copy-message")
		_, result := automationdriver.FindByAutomationIDPrefix(snapshot, "action-result-hide-launcher-")
		return found && status.Value == message && status.Role == woxui.AccessibilityRoleButton && status.Expanded == open && keep == open && clear == open && copy == open && !result
	})
	if err != nil {
		status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
		t.Fatalf("wait for toolbar message %q panel open=%t: status found=%t value=%q expanded=%t: %v", message, open, found, status.Value, status.Expanded, err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
	return snapshot
}
