//go:build wox_ui_smoke

package storestatus

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test002LauncherStoreInstallFailureToolbar verifies a broken store package reports the failure in WPM.
// Flow: find a local catalog entry -> install its invalid archive -> leave the WPM query.
// Evidence: the toolbar names the plugin and archive error, then clears after leaving WPM.
func Test002LauncherStoreInstallFailureToolbar(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		const name = "Store Failure Smoke"
		userDir := os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment)
		if userDir == "" || !filepath.IsAbs(userDir) {
			t.Fatal("isolated smoke user data directory is not configured")
		}
		partialDir := filepath.Join(userDir, "plugins", "com.wox.smoke.storefailure@1.0.0")
		t.Cleanup(func() { _ = os.RemoveAll(partialDir) })
		smoke.ShowLauncher(t, ctx, client)
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, "wpm install "+name)
			if id, found := smoke.FindLauncherResult(snapshot, name); found {
				smoke.SelectLauncherResult(t, ctx, client, id)
				if err := client.Perform(ctx, id, woxui.AccessibilityActionActivate, ""); err != nil {
					t.Fatal(err)
				}
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal("invalid package store result did not appear")
			case <-ticker.C:
			}
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return found && strings.Contains(status.Value, name) && strings.Contains(strings.ToLower(status.Value), "zip")
		})
		if err != nil {
			status, _ := automationdriver.Find(snapshot, "launcher.toolbar.status")
			t.Fatalf("wait for store install error: %q: %v", status.Value, err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		smoke.ReplaceLauncherQuery(t, ctx, client, "1+1")
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return !found || !strings.Contains(status.Value, name)
		}); err != nil {
			t.Fatalf("store error survived leaving WPM: %v", err)
		}
	})
}
