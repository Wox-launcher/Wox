//go:build wox_ui_smoke

package wpm

import (
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

// Test002LauncherPluginWpmUninstallToolbar verifies the uninstall outcome remains in the WPM toolbar.
// Flow: load a local single-file plugin -> uninstall it through WPM -> leave the WPM query.
// Evidence: the plugin file is removed, its named success status is visible, and the status clears on exit.
func Test002LauncherPluginWpmUninstallToolbar(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		const name = "WPM Toolbar Smoke Plugin"
		userDir := os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment)
		if userDir == "" || !filepath.IsAbs(userDir) {
			t.Fatal("isolated smoke user data directory is not configured")
		}
		directory := filepath.Join(userDir, "plugins", "single-file")
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(directory, "Wox.Plugin.WpmToolbarSmoke.js")
		const source = `// {"Id":"com.wox.smoke.wpmtoolbar","Name":"WPM Toolbar Smoke Plugin","Version":"1.0.0","MinWoxVersion":"2.4.2","Runtime":"NODEJS","TriggerKeywords":["wpmtoolbarfixture"],"SupportedOS":["Windows","Linux","Macos"]}
class SmokePlugin {
  async init(ctx, params) {}
  async query(ctx, query) { return { Results: [{ Title: "WPM Toolbar Fixture", Actions: [] }] } }
}
module.exports.plugin = new SmokePlugin()
`
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(path) })
		smoke.ShowLauncher(t, ctx, client)
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, "wpmtoolbarfixture ")
			if _, found := smoke.FindLauncherResult(snapshot, "WPM Toolbar Fixture"); found {
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal("single-file fixture did not load")
			case <-ticker.C:
			}
		}
		snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, "wpm uninstall WPM Toolbar Smoke Plugin")
		id, found := smoke.FindLauncherResult(snapshot, name)
		if !found {
			t.Fatalf("uninstall result for %q not found", name)
		}
		if err := client.Perform(ctx, id, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("uninstall fixture: %v", err)
		}
		success := map[string]bool{}
		for _, lang := range i18n.GetSupportedLanguages() {
			texts, err := i18n.GetI18nManager().GetLangMap(ctx, lang.Code)
			if err != nil {
				t.Fatal(err)
			}
			success[texts["plugin_installer_verb_uninstall_past"]] = true
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			if !found || !strings.Contains(status.Value, name) {
				return false
			}
			for verb := range success {
				if verb != "" && strings.Contains(status.Value, verb) {
					_, err := os.Stat(path)
					return os.IsNotExist(err)
				}
			}
			return false
		})
		if err != nil {
			status, _ := automationdriver.Find(snapshot, "launcher.toolbar.status")
			t.Fatalf("wait for uninstall status and file removal: status=%q: %v", status.Value, err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		smoke.ReplaceLauncherQuery(t, ctx, client, "1+1")
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return !found || !strings.Contains(status.Value, name)
		}); err != nil {
			t.Fatalf("WPM status did not clear after leaving query: %v", err)
		}
	})
}
