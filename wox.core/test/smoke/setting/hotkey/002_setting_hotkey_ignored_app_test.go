//go:build wox_ui_smoke

package hotkey

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

const ignoredHotkeyAppsFieldID = "hotkey-settings-field-2"

// Test002SettingHotkeyIgnoredApp verifies that a configured foreground system editor suppresses the registered main hotkey.
// Flow: select the platform editor in Ignore Hotkey Apps -> focus a new editor process -> press the registered Ctrl+F12 hotkey.
// Evidence: Wox logs the matched application identity and the real launcher remains hidden.
func Test002SettingHotkeyIgnoredApp(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		requireIgnoredHotkeyAppRuntime(t)
		appQuery, appIdentity := ignoredHotkeyAppTarget(t)
		hotkey := ignoredHotkeyAppHotkey(t)
		logPath := filepath.Join(os.Getenv(automationdriver.SharedDataDirectoryEnvironment), "log", "wox.log")
		smoke.WaitForApplicationCatalog(t, ctx, logPath)
		if current := openMainHotkeySettings(t, ctx, client); current != hotkey {
			t.Fatalf("isolated main hotkey = %q, want platform default %q", current, hotkey)
		}
		addedRow := addIgnoredHotkeyApp(t, ctx, client, appQuery)
		t.Cleanup(func() { removeIgnoredHotkeyApp(t, client, addedRow) })

		if err := client.Hide(ctx); err != nil {
			t.Fatalf("close Hotkey settings before activating ignored app: %v", err)
		}
		activateIgnoredHotkeyApp(t, ctx)
		logOffset := currentHotkeyLogSize(t, logPath)
		sendIgnoredAppNativeHotkey(t, hotkey)

		logs := waitForHotkeyLog(t, ctx, logPath, logOffset, "ignore hotkey trigger for app identity="+appIdentity)
		state, err := client.WindowState(ctx, "primary")
		if err != nil {
			t.Fatalf("read launcher state after ignored hotkey: %v", err)
		}
		if state.Visible {
			t.Fatalf("launcher became visible after ignored app hotkey; logs:\n%s", logs)
		}
	})
}

// addIgnoredHotkeyApp selects one indexed application and waits for its settings row to persist.
func addIgnoredHotkeyApp(t *testing.T, ctx context.Context, client *automationdriver.Client, query string) int {
	t.Helper()
	openIgnoredHotkeyApps(t, ctx, client)
	return smoke.AddApplicationTableRow(t, ctx, client, ignoredHotkeyAppsFieldID, query)
}

// openIgnoredHotkeyApps navigates to the inline table through the localized Settings search.
func openIgnoredHotkeyApps(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.OpenSettings(ctx, "/general"); err != nil {
		t.Fatalf("open General settings: %v", err)
	}
	if err := client.Perform(ctx, "settings-search-field", woxui.AccessibilityActionSetValue, "IgnoredHotkeyApps"); err != nil {
		t.Fatalf("search for Ignore Hotkey Apps: %v", err)
	}
	if err := client.PressKey(ctx, woxui.KeyEnter, 0); err != nil {
		t.Fatalf("open Ignore Hotkey Apps from search: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, pageFound := automationdriver.Find(snapshot, "settings.page.general")
		_, addFound := automationdriver.Find(snapshot, ignoredHotkeyAppsFieldID+"-add")
		return pageFound && addFound
	}); err != nil {
		t.Fatalf("wait for Ignore Hotkey Apps setting: %v", err)
	}
}

// removeIgnoredHotkeyApp removes the row added by this case and waits for persistence to finish.
func removeIgnoredHotkeyApp(t *testing.T, client *automationdriver.Client, rowIndex int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := client.Hide(ctx); err != nil {
		t.Errorf("hide active window before removing ignored hotkey app: %v", err)
	}
	openIgnoredHotkeyApps(t, ctx, client)
	smoke.RemoveApplicationTableRow(t, ctx, client, ignoredHotkeyAppsFieldID, rowIndex)
	if err := client.Hide(ctx); err != nil {
		t.Errorf("close settings after removing ignored hotkey app: %v", err)
	}
}

// currentHotkeyLogSize captures the boundary before the native hotkey is injected.
func currentHotkeyLogSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat Wox log %q: %v", path, err)
	}
	return info.Size()
}

// waitForHotkeyLog waits for runtime evidence written after the captured log boundary.
func waitForHotkeyLog(t *testing.T, ctx context.Context, path string, offset int64, expected string) string {
	t.Helper()
	data := smoke.WaitForFile(t, ctx, path, func(data []byte) bool {
		return int64(len(data)) >= offset && strings.Contains(string(data[offset:]), expected)
	})
	return string(data[offset:])
}
