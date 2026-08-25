//go:build wox_ui_smoke

package query

import (
	"context"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const toolbarSmokeQuery = "wox-smoke toolbar "

// Test013LauncherQueryToolbarMessage verifies a plugin-owned toolbar message follows query and action lifecycle changes.
// Flow: enter the toolbar fixture query -> activate Keep open -> leave and re-enter the query -> activate Clear.
// Evidence: the fixture status and progress update, leave on query change, reappear, and then clear. A launcher-wide fallback may remain.
func Test013LauncherQueryToolbarMessage(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		waitForToolbarMessage(t, ctx, client)
		activateToolbarFixtureAction(t, ctx, client, "toolbar-action-toolbar-keep-open-", "action-toolbar-keep-open-")
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
		waitForToolbarFixtureGone(t, ctx, client, "wait for toolbar fixture to leave after query change")

		waitForToolbarMessage(t, ctx, client)
		activateToolbarFixtureAction(t, ctx, client, "toolbar-action-toolbar-clear-", "action-toolbar-clear-")
		waitForToolbarFixtureGone(t, ctx, client, "wait for toolbar fixture to clear")
	})
}

// waitForToolbarMessage enters the fixture command and waits for its complete toolbar semantics.
func waitForToolbarMessage(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, toolbarSmokeQuery)
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		status, statusFound := automationdriver.Find(snapshot, "launcher.toolbar.status")
		progress, progressFound := automationdriver.Find(snapshot, "launcher.toolbar.progress")
		return statusFound && status.Value == "Toolbar fixture ready" && progressFound && progress.Value == "loading"
	})
	if err != nil {
		t.Fatalf("wait for toolbar fixture message: %v", err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
}

// activateToolbarFixtureAction uses the footer chip when it still fits, otherwise the action panel.
func activateToolbarFixtureAction(t *testing.T, ctx context.Context, client *automationdriver.Client, footerPrefix, panelPrefix string) {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read toolbar actions for %q: %v", footerPrefix, err)
	}
	if action, found := automationdriver.FindByAutomationIDPrefix(snapshot, footerPrefix); found {
		if err := client.Perform(ctx, action.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("activate toolbar footer action %q: %v", footerPrefix, err)
		}
		return
	}
	smoke.ActivateSelectedResultAction(t, ctx, client, panelPrefix)
}

// waitForToolbarFixtureGone waits until the smoke fixture status is gone. A
// launcher-wide fallback, such as a main-hotkey registration warning, may keep
// occupying the footer after the plugin-owned message leaves.
func waitForToolbarFixtureGone(t *testing.T, ctx context.Context, client *automationdriver.Client, message string) {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		return !toolbarFixtureMessageVisible(snapshot)
	})
	if err != nil {
		status, statusFound := automationdriver.Find(snapshot, "launcher.toolbar.status")
		t.Fatalf("%s: leftover status found=%t value=%q: %v", message, statusFound, status.Value, err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
}

func toolbarFixtureMessageVisible(snapshot woxwidget.AutomationSnapshot) bool {
	status, statusFound := automationdriver.Find(snapshot, "launcher.toolbar.status")
	return statusFound && (status.Value == "Toolbar fixture ready" || strings.HasPrefix(status.Value, "Toolbar fixture keep-open:"))
}
