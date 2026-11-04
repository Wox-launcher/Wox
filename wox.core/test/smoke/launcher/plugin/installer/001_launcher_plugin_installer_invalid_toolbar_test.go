//go:build wox_ui_smoke

package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test001LauncherPluginInstallerInvalidToolbar verifies an invalid local package reports its error in context.
// Flow: select an invalid .wox file -> inspect the installer result and toolbar -> leave the selection window.
// Evidence: both the selected result and the toolbar identify the package, and the message leaves with its context.
func Test001LauncherPluginInstallerInvalidToolbar(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		path := filepath.Join(t.TempDir(), "broken-toolbar-package.wox")
		if err := os.WriteFile(path, []byte("not a zip archive"), 0o644); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		if err := client.OpenFileSelectionQuery(ctx, []string{path}); err != nil {
			t.Fatalf("open local package selection: %v", err)
		}
		if _, err := client.WaitForWindowState(ctx, "selection", func(state automationdriver.WindowState) bool {
			return state.Exists && state.Visible
		}); err != nil {
			t.Fatalf("wait for package selection: %v", err)
		}
		if err := client.FocusInstance(ctx, "selection"); err != nil {
			t.Fatal(err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			if !found || !strings.Contains(status.Value, filepath.Base(path)) {
				return false
			}
			for _, node := range snapshot.Tree.Nodes {
				if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.Contains(node.Label, filepath.Base(path)) && node.Selected {
					return true
				}
			}
			return false
		})
		if err != nil {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			t.Fatalf("wait for invalid package result and status: found=%t value=%q: %v", found, status.Value, err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		smoke.ShowLauncher(t, ctx, client)
		if err := client.FocusInstance(ctx, "primary"); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return !found || !strings.Contains(status.Value, filepath.Base(path))
		}); err != nil {
			t.Fatalf("package status survived leaving selection: %v", err)
		}
	})
}
