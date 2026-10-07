//go:build wox_ui_smoke

package offline

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test001SettingOfflineMode verifies offline policy replaces online settings pages and restores them.
// Flow: enable offline mode in Privacy -> visit both stores and Cloud Sync -> follow recovery -> disable offline.
// Evidence: each page exposes only offline recovery while paused, then its normal controls return without requiring network data.
func Test001SettingOfflineMode(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		openPrivacySettings(t, ctx, client)
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatal(err)
		}
		initial, found := automationdriver.Find(snapshot, "privacy-offline-switch")
		if !found {
			t.Fatal("offline switch missing")
		}
		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), automationdriver.ActionTimeout)
			defer cancel()
			setOfflineModeForSmoke(t, cleanupCtx, client, initial.Checked)
		})
		setOfflineModeForSmoke(t, ctx, client, true)
		pages := []struct{ route, page, control string }{
			{"/plugins/store", "plugins", "plugin-search"},
			{"/themes/store", "theme", "theme-search"},
			{"/cloud", "cloud", "cloud-login"},
		}
		for _, page := range pages {
			if err := client.OpenSettings(ctx, page.route); err != nil {
				t.Fatal(err)
			}
			snapshot, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
				_, pageFound := automationdriver.Find(s, "settings.page."+page.page)
				_, offline := automationdriver.Find(s, "offline-open-settings")
				_, normal := automationdriver.Find(s, page.control)
				return pageFound && offline && !normal
			})
			if err != nil {
				t.Fatalf("offline page %s: %v", page.route, err)
			}
			smoke.AssertNoDiagnostics(t, snapshot)
		}
		if err := client.Perform(ctx, "offline-open-settings", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
			control, found := automationdriver.Find(s, "privacy-offline-switch")
			return found && control.Checked
		}); err != nil {
			t.Fatalf("recovery did not open Privacy with offline enabled: %v", err)
		}
		setOfflineModeForSmoke(t, ctx, client, false)
		for _, page := range pages {
			if err := client.OpenSettings(ctx, page.route); err != nil {
				t.Fatal(err)
			}
			snapshot, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
				_, pageFound := automationdriver.Find(s, "settings.page."+page.page)
				_, offline := automationdriver.Find(s, "offline-open-settings")
				_, normal := automationdriver.Find(s, page.control)
				return pageFound && !offline && normal
			})
			if err != nil {
				t.Fatalf("restored page %s: %v", page.route, err)
			}
			smoke.AssertNoDiagnostics(t, snapshot)
		}
	})
}

// setOfflineModeForSmoke uses the public switch and reopens its page to observe committed state.
func setOfflineModeForSmoke(t *testing.T, ctx context.Context, client *automationdriver.Client, enabled bool) {
	t.Helper()
	openPrivacySettings(t, ctx, client)
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	control, found := automationdriver.Find(snapshot, "privacy-offline-switch")
	if !found {
		t.Fatal("offline switch missing")
	}
	if control.Checked != enabled {
		if err := client.Perform(ctx, "privacy-offline-switch", woxui.AccessibilityActionToggle, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
		control, found := automationdriver.Find(s, "privacy-offline-switch")
		return found && control.Checked == enabled
	}); err != nil {
		t.Fatalf("offline=%t: %v", enabled, err)
	}
	openPrivacySettings(t, ctx, client)
}

// openPrivacySettings waits for the owning page before interacting with its offline switch.
func openPrivacySettings(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.OpenSettings(ctx, "/privacy"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
		_, page := automationdriver.Find(s, "settings.page.privacy")
		_, control := automationdriver.Find(s, "privacy-offline-switch")
		return page && control
	}); err != nil {
		t.Fatalf("open Privacy: %v", err)
	}
}
