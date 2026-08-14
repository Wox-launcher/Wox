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

const toolbarSmokeQuery = "wox-smoke toolbar "

// Test013LauncherQueryToolbarMessage verifies a plugin-owned toolbar message follows query and action lifecycle changes.
// Flow: enter the toolbar fixture query -> activate Keep open -> leave and re-enter the query -> activate Clear.
// Evidence: the toolbar status and progress semantics update, disappear on query change, reappear, and then clear.
func Test013LauncherQueryToolbarMessage(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		waitForToolbarMessage(t, ctx, client)

		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read toolbar fixture semantics: %v", err)
		}
		keepOpen, found := automationdriver.FindByAutomationIDPrefix(snapshot, "toolbar-action-toolbar-keep-open-")
		if !found {
			t.Fatal("toolbar Keep open action was not exposed")
		}
		if err := client.Perform(ctx, keepOpen.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("activate toolbar Keep open action: %v", err)
		}
		updated, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, statusFound := automationdriver.Find(snapshot, "launcher.toolbar.status")
			_, queryFound := automationdriver.Find(snapshot, "launcher.query.input")
			return statusFound && status.Value == "Toolbar fixture keep-open: context-round-trip" && queryFound
		})
		if err != nil {
			t.Fatalf("wait for toolbar Keep open update: %v", err)
		}
		smoke.AssertNoDiagnostics(t, updated)

		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "1+1")
		clearedOnQueryChange, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, statusFound := automationdriver.Find(snapshot, "launcher.toolbar.status")
			_, progressFound := automationdriver.Find(snapshot, "launcher.toolbar.progress")
			return !statusFound && !progressFound
		})
		if err != nil {
			t.Fatalf("wait for toolbar message to clear on query change: %v", err)
		}
		smoke.AssertNoDiagnostics(t, clearedOnQueryChange)

		waitForToolbarMessage(t, ctx, client)
		snapshot, err = client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read toolbar semantics before clear: %v", err)
		}
		clear, found := automationdriver.FindByAutomationIDPrefix(snapshot, "toolbar-action-toolbar-clear-")
		if !found {
			t.Fatal("toolbar Clear action was not exposed")
		}
		if err := client.Perform(ctx, clear.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("activate toolbar Clear action: %v", err)
		}
		cleared, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, statusFound := automationdriver.Find(snapshot, "launcher.toolbar.status")
			_, progressFound := automationdriver.Find(snapshot, "launcher.toolbar.progress")
			return !statusFound && !progressFound
		})
		if err != nil {
			t.Fatalf("wait for toolbar message to clear: %v", err)
		}
		smoke.AssertNoDiagnostics(t, cleared)
	})
}

// waitForToolbarMessage enters the fixture command and waits for its complete toolbar semantics.
func waitForToolbarMessage(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, toolbarSmokeQuery)
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		status, statusFound := automationdriver.Find(snapshot, "launcher.toolbar.status")
		progress, progressFound := automationdriver.Find(snapshot, "launcher.toolbar.progress")
		_, keepOpenFound := automationdriver.FindByAutomationIDPrefix(snapshot, "toolbar-action-toolbar-keep-open-")
		_, clearFound := automationdriver.FindByAutomationIDPrefix(snapshot, "toolbar-action-toolbar-clear-")
		return statusFound && status.Value == "Toolbar fixture ready" && progressFound && progress.Value == "loading" && keepOpenFound && clearFound
	})
	if err != nil {
		t.Fatalf("wait for toolbar fixture message: %v", err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
}
