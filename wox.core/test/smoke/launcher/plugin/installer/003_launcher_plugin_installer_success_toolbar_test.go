//go:build wox_ui_smoke

package installer

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wox/i18n"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test003LauncherPluginInstallerSuccessToolbar verifies local installation and upgrade report their outcomes.
// Flow: install a local plugin package -> select its newer package -> upgrade -> uninstall the fixture through WPM.
// Evidence: each version's files and localized status appear, then the fixture is removed.
func Test003LauncherPluginInstallerSuccessToolbar(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		const id, name = "com.wox.smoke.localtoolbar", "Local Toolbar Smoke"
		userDir := os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment)
		if userDir == "" || !filepath.IsAbs(userDir) {
			t.Fatal("isolated smoke user data directory is not configured")
		}
		pluginRoot := filepath.Join(userDir, "plugins")
		for _, version := range []string{"1.0.0", "1.0.1"} {
			versionDir := filepath.Join(pluginRoot, id+"@"+version)
			t.Cleanup(func() { _ = os.RemoveAll(versionDir) })
		}
		createPackage := func(version string) string {
			path := filepath.Join(t.TempDir(), "local-"+version+".wox")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			archive := zip.NewWriter(file)
			manifest, err := archive.Create("plugin.json")
			if err != nil {
				t.Fatal(err)
			}
			metadata := fmt.Sprintf(`{"Id":%q,"Name":%q,"Version":%q,"MinWoxVersion":"2.0.0","Runtime":"NODEJS","Entry":"index.js","TriggerKeywords":["localtoolbarsmoke"],"SupportedOS":["Windows","Linux","Darwin"]}`, id, name, version)
			if _, err := manifest.Write([]byte(metadata)); err != nil {
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
			return path
		}
		install := func(path, version, verbKey string) {
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
				t.Fatalf("wait for %s package: %v", version, err)
			}
			for _, node := range snapshot.Tree.Nodes {
				if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.Contains(node.Label, name) && node.Selected {
					if err := client.Perform(ctx, node.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
						t.Fatal(err)
					}
					break
				}
			}
			verbs := map[string]bool{}
			for _, lang := range i18n.GetSupportedLanguages() {
				texts, err := i18n.GetI18nManager().GetLangMap(ctx, lang.Code)
				if err != nil {
					t.Fatal(err)
				}
				verbs[texts[verbKey]] = true
			}
			snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
				if !found || !strings.Contains(status.Value, name) {
					return false
				}
				for verb := range verbs {
					if verb != "" && strings.Contains(status.Value, verb) {
						_, err := os.Stat(filepath.Join(pluginRoot, id+"@"+version, "plugin.json"))
						return err == nil
					}
				}
				return false
			})
			if err != nil {
				status, _ := automationdriver.Find(snapshot, "launcher.toolbar.status")
				t.Fatalf("wait for %s status: %q: %v", version, status.Value, err)
			}
			smoke.AssertNoDiagnostics(t, snapshot)
			if err := client.FocusInstance(ctx, "primary"); err != nil {
				t.Fatal(err)
			}
		}
		install(createPackage("1.0.0"), "1.0.0", "plugin_installer_verb_install_past")
		install(createPackage("1.0.1"), "1.0.1", "plugin_installer_verb_upgrade_past")
		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, "wpm uninstall "+name)
		result, found := smoke.FindLauncherResult(snapshot, name)
		if !found {
			t.Fatal("installed plugin not found in WPM")
		}
		if err := client.Perform(ctx, result, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			_, err := os.Stat(filepath.Join(pluginRoot, id+"@1.0.1"))
			return found && strings.Contains(status.Value, name) && os.IsNotExist(err)
		}); err != nil {
			t.Fatalf("fixture uninstall did not complete: %v", err)
		}
	})
}
