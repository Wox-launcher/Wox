//go:build wox_ui_smoke

package storestatus

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wox/i18n"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test001LauncherStoreToolbar verifies store installation and both upgrade entry points report status.
// Flow: install from WPM -> install an older local package -> upgrade from WPM -> downgrade -> upgrade from the indicator -> uninstall.
// Evidence: each operation displays the named outcome and the corresponding real plugin files change version.
func Test001LauncherStoreToolbar(t *testing.T) {
	smoke.CaseWithTimeout(t, 2*time.Minute, func(ctx context.Context, client *automationdriver.Client) {
		const name, id = "Store Toolbar Smoke", "com.wox.smoke.storetoolbar"
		userDir := os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment)
		if userDir == "" || !filepath.IsAbs(userDir) {
			t.Fatal("isolated smoke user data directory is not configured")
		}
		pluginDir := filepath.Join(userDir, "plugins")
		for _, version := range []string{"1.0.0", "1.0.1"} {
			dir := filepath.Join(pluginDir, id+"@"+version)
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
		}
		labels := map[string]map[string]bool{}
		for _, key := range []string{"plugin_wpm_install", "plugin_wpm_upgrade", "plugin_wpm_uninstall", "plugin_installer_verb_install_past", "plugin_installer_verb_upgrade_past", "plugin_installer_verb_uninstall_past"} {
			labels[key] = map[string]bool{}
		}
		for _, lang := range i18n.GetSupportedLanguages() {
			texts, err := i18n.GetI18nManager().GetLangMap(ctx, lang.Code)
			if err != nil {
				t.Fatal(err)
			}
			for key := range labels {
				labels[key][texts[key]] = true
			}
		}
		waitResult := func(query, title string) {
			ticker := time.NewTicker(250 * time.Millisecond)
			defer ticker.Stop()
			for {
				snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, query)
				if id, found := smoke.FindLauncherResult(snapshot, title); found {
					smoke.SelectLauncherResult(t, ctx, client, id)
					return
				}
				select {
				case <-ctx.Done():
					t.Fatalf("wait for store result %q", title)
				case <-ticker.C:
				}
			}
		}
		activate := func(key string) {
			if err := client.Perform(ctx, "result-toolbar-more", woxui.AccessibilityActionActivate, ""); err != nil {
				t.Fatal(err)
			}
			snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				for _, node := range snapshot.Tree.Nodes {
					if strings.HasPrefix(node.AutomationID, "action-result-") && labels[key][node.Label] {
						return true
					}
				}
				return false
			})
			if err != nil {
				t.Fatalf("wait for action %s: %v", key, err)
			}
			for _, node := range snapshot.Tree.Nodes {
				if strings.HasPrefix(node.AutomationID, "action-result-") && labels[key][node.Label] {
					if err := client.Perform(ctx, node.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
						t.Fatal(err)
					}
					return
				}
			}
		}
		waitOutcome := func(verbKey, version string, exists bool) {
			snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
				if !found || !strings.Contains(status.Value, name) {
					return false
				}
				for verb := range labels[verbKey] {
					if verb != "" && strings.Contains(status.Value, verb) {
						_, err := os.Stat(filepath.Join(pluginDir, id+"@"+version))
						return (err == nil) == exists
					}
				}
				return false
			})
			if err != nil {
				status, _ := automationdriver.Find(snapshot, "launcher.toolbar.status")
				t.Fatalf("wait for %s outcome: %q: %v", verbKey, status.Value, err)
			}
			smoke.AssertNoDiagnostics(t, snapshot)
		}
		packagePath := filepath.Join(t.TempDir(), "older.wox")
		file, err := os.Create(packagePath)
		if err != nil {
			t.Fatal(err)
		}
		archive := zip.NewWriter(file)
		manifest, err := archive.Create("plugin.json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := manifest.Write([]byte(`{"Id":"com.wox.smoke.storetoolbar","Name":"Store Toolbar Smoke","Version":"1.0.0","MinWoxVersion":"2.0.0","Runtime":"NODEJS","Entry":"index.js","TriggerKeywords":["storetoolbarsmoke"],"SupportedOS":["Windows","Linux","Darwin"]}`)); err != nil {
			t.Fatal(err)
		}
		entry, err := archive.Create("index.js")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(`module.exports.plugin = { async init(ctx, params) {}, async query(ctx, query) { return { Results: [] } } }`)); err != nil {
			t.Fatal(err)
		}
		if err := archive.Close(); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		installOlder := func() {
			smoke.ShowLauncher(t, ctx, client)
			if err := client.OpenFileSelectionQuery(ctx, []string{packagePath}); err != nil {
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
				t.Fatalf("wait for older package: %v", err)
			}
			for _, node := range snapshot.Tree.Nodes {
				if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.Contains(node.Label, name) && node.Selected {
					if err := client.Perform(ctx, node.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
						t.Fatal(err)
					}
					break
				}
			}
			if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
				_, err := os.Stat(filepath.Join(pluginDir, id+"@1.0.0", "plugin.json"))
				return found && strings.Contains(status.Value, name) && err == nil
			}); err != nil {
				t.Fatalf("wait for older package install: %v", err)
			}
			if err := client.FocusInstance(ctx, "primary"); err != nil {
				t.Fatal(err)
			}
		}
		smoke.ShowLauncher(t, ctx, client)
		waitResult("wpm install "+name, name)
		activate("plugin_wpm_install")
		waitOutcome("plugin_installer_verb_install_past", "1.0.1", true)
		installOlder()
		smoke.ShowLauncher(t, ctx, client)
		waitResult("wpm install "+name, name)
		activate("plugin_wpm_upgrade")
		waitOutcome("plugin_installer_verb_upgrade_past", "1.0.1", true)
		installOlder()
		smoke.ShowLauncher(t, ctx, client)
		waitResult("storetoolbarsmoke", "storetoolbarsmoke")
		activate("plugin_wpm_upgrade")
		waitOutcome("plugin_installer_verb_upgrade_past", "1.0.1", true)
		waitResult("wpm uninstall "+name, name)
		activate("plugin_wpm_uninstall")
		waitOutcome("plugin_installer_verb_uninstall_past", "1.0.1", false)
	})
}
