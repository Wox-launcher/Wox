//go:build wox_ui_smoke

package installer

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test002LauncherPluginInstallerIncompatibleToolbar verifies a local install failure is reported in the active selection.
// Flow: select a package with an invalid minimum Wox version -> execute Install -> inspect the toolbar.
// Evidence: the failure identifies the plugin and invalid version, and no plugin files are installed.
func Test002LauncherPluginInstallerIncompatibleToolbar(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		const name = "Incompatible Toolbar Smoke"
		path := filepath.Join(t.TempDir(), "incompatible.wox")
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		archive := zip.NewWriter(file)
		entry, err := archive.Create("plugin.json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(`{"Id":"com.wox.smoke.incompatible","Name":"Incompatible Toolbar Smoke","Version":"1.0.0","MinWoxVersion":"invalid-version-toolbar","Runtime":"NODEJS","TriggerKeywords":["incompatibletoolbar"],"SupportedOS":["Windows","Linux","Macos"]}`)); err != nil {
			t.Fatal(err)
		}
		if err := archive.Close(); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		if err := client.OpenFileSelectionQuery(ctx, []string{path}); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitForWindowState(ctx, "selection", func(state automationdriver.WindowState) bool { return state.Exists && state.Visible }); err != nil {
			t.Fatal(err)
		}
		if err := client.FocusInstance(ctx, "selection"); err != nil {
			t.Fatal(err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			for _, node := range snapshot.Tree.Nodes {
				if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.Contains(node.Label, name) && node.Selected {
					return true
				}
			}
			return false
		})
		if err != nil {
			t.Fatalf("wait for local install result: %v", err)
		}
		var resultID string
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.Contains(node.Label, name) && node.Selected {
				resultID = node.AutomationID
				break
			}
		}
		if err := client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return found && strings.Contains(status.Value, name) && strings.Contains(status.Value, "invalid-version-toolbar")
		})
		if err != nil {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			t.Fatalf("wait for incompatible install status: found=%t value=%q: %v", found, status.Value, err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		installed := filepath.Join(os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment), "plugins", "com.wox.smoke.incompatible@1.0.0")
		if _, err := os.Stat(installed); !os.IsNotExist(err) {
			t.Fatalf("incompatible package unexpectedly installed at %q: %v", installed, err)
		}
	})
}
