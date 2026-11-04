//go:build wox_ui_smoke

package filesearch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/clipboard"
)

// Test009LauncherPluginFileSearchErrorToolbar verifies an index error offers its real recovery actions.
// Flow: deny access to a temporary index root -> open File Search -> copy the error -> open plugin settings.
// Evidence: the toolbar identifies the root, the clipboard contains its error, and File Search settings opens.
func Test009LauncherPluginFileSearchErrorToolbar(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.PreserveClipboard(t)
		root := filepath.Join(newFileSearchRoot(t), "toolbar-denied-root")
		if err := os.Mkdir(root, 0o755); err != nil {
			t.Fatal(err)
		}
		denyFileSearchRoot(t, root)
		row := addFileSearchRoot(t, ctx, client, root)
		t.Cleanup(func() { removeFileSearchRoot(t, client, root, row) })
		smoke.ShowLauncher(t, ctx, client)
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "f status ")
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return found && strings.Contains(status.Value, "toolbar-denied-root") && status.Role == woxui.AccessibilityRoleButton
		})
		if err != nil {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			t.Fatalf("wait for index error: found=%t value=%q: %v", found, status.Value, err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		openActions := func() woxwidget.AutomationSnapshot {
			if err := client.Perform(ctx, "launcher.toolbar.status", woxui.AccessibilityActionActivate, ""); err != nil {
				t.Fatal(err)
			}
			snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				_, copy := automationdriver.FindByAutomationIDPrefix(snapshot, "action-toolbar-copy-error")
				_, settings := automationdriver.FindByAutomationIDPrefix(snapshot, "action-toolbar-open-settings")
				return copy && settings
			})
			if err != nil {
				t.Fatalf("wait for index error actions: %v", err)
			}
			return snapshot
		}
		snapshot = openActions()
		copyAction, _ := automationdriver.FindByAutomationIDPrefix(snapshot, "action-toolbar-copy-error")
		if err := client.Perform(ctx, copyAction.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		if text, err := clipboard.ReadText(); err != nil || !strings.Contains(text, root) || !strings.Contains(text, "\n") {
			t.Fatalf("copied index details = %q, err=%v", text, err)
		}
		snapshot = openActions()
		settingsAction, _ := automationdriver.FindByAutomationIDPrefix(snapshot, "action-toolbar-open-settings")
		if err := client.Perform(ctx, settingsAction.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := automationdriver.Find(snapshot, fileSearchRootsFieldID+"-add")
			return found
		}); err != nil {
			t.Fatalf("File Search settings did not open: %v", err)
		}
	})
}
