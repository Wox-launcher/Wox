//go:build wox_ui_smoke

package data

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test001SettingDataLogLevel verifies the Data page log-level choice menu.
func Test001SettingDataLogLevel(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		if err := client.OpenSettings(ctx, "/data"); err != nil {
			t.Fatalf("open Data settings: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, windowFound := automationdriver.Find(snapshot, "settings.window")
			_, pageFound := automationdriver.Find(snapshot, "settings.page.data")
			_, logLevelFound := automationdriver.Find(snapshot, "data-log-level")
			return windowFound && pageFound && logLevelFound
		}); err != nil {
			t.Fatalf("wait for Data settings: %v", err)
		}
		if err := client.Perform(ctx, "data-log-level", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("open log level dropdown: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, menuFound := automationdriver.Find(snapshot, "setting-choice-menu")
			info, infoFound := automationdriver.Find(snapshot, "setting-choice-0")
			debug, debugFound := automationdriver.Find(snapshot, "setting-choice-1")
			return menuFound && infoFound && debugFound && info.Label == "INFO" && debug.Label == "DEBUG"
		}); err != nil {
			t.Fatalf("wait for log level dropdown choices: %v", err)
		}
		capturePath := smoke.ArtifactPath(t, "setting-data-001-log-level")
		if err := client.Capture(ctx, capturePath); err != nil {
			t.Fatalf("capture log level dropdown: %v", err)
		}
		smoke.AssertPNG(t, capturePath)
	})
}
