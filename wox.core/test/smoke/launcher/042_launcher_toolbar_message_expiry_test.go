//go:build wox_ui_smoke

package query

import (
	"context"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test042LauncherToolbarMessageExpiry verifies default expiry and explicit persistence.
// Flow: show a transient message -> wait for expiry -> show a persistent message -> clear it.
// Evidence: the transient status disappears, while the persistent status survives 10 seconds and clears on action.
func Test042LauncherToolbarMessageExpiry(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "wox-smoke toolbar-long-plain ")
		transientVisible := func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return found && strings.HasPrefix(status.Value, "Indexing file contents across")
		}
		if _, err := client.WaitFor(ctx, transientVisible); err != nil {
			t.Fatalf("wait for transient toolbar message: %v", err)
		}
		// Split the observation into two bounded waits so the expiry itself has a full 10-second budget.
		start := time.Now()
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			return transientVisible(snapshot) && time.Since(start) >= 2*time.Second
		}); err != nil {
			t.Fatalf("transient toolbar message did not remain visible initially: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool { return !transientVisible(snapshot) })
		if err != nil {
			t.Fatalf("wait for transient toolbar message to expire: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)

		waitForToolbarMessage(t, ctx, client)
		start = time.Now()
		for _, duration := range []time.Duration{9 * time.Second, 11 * time.Second} {
			_, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				if !toolbarFixtureMessageVisible(snapshot) {
					t.Fatal("persistent toolbar message disappeared before it was cleared")
				}
				return time.Since(start) >= duration
			})
			if err != nil {
				t.Fatalf("observe persistent toolbar message for %s: %v", duration, err)
			}
		}
		activateToolbarFixtureAction(t, ctx, client, "toolbar-action-toolbar-clear-", "action-toolbar-clear-")
		waitForToolbarFixtureGone(t, ctx, client, "wait for persistent message to clear")
	})
}
