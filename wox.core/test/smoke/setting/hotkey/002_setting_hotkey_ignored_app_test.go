//go:build wox_ui_smoke

package hotkey

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
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
		waitForApplicationIndex(t, ctx, logPath)
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

// waitForApplicationIndex waits until Settings can cache the complete platform application catalog.
func waitForApplicationIndex(t *testing.T, ctx context.Context, path string) {
	t.Helper()
	smoke.WaitForFile(t, ctx, path, func(data []byte) bool {
		logs := string(data)
		return strings.Contains(logs, "[Apps] indexed ") && strings.Contains(logs, " apps, cost ")
	})
}

// addIgnoredHotkeyApp selects one indexed application and waits for its settings row to persist.
func addIgnoredHotkeyApp(t *testing.T, ctx context.Context, client *automationdriver.Client, query string) int {
	t.Helper()
	openIgnoredHotkeyApps(t, ctx, client)
	rowIndex := ignoredHotkeyAppRowCount(t, ctx, client)
	if err := client.Perform(ctx, ignoredHotkeyAppsFieldID+"-add", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("add ignored hotkey app: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, fieldFound := automationdriver.Find(snapshot, "form-table-row-field-0")
		_, saveFound := automationdriver.Find(snapshot, "form-table-row-save")
		return fieldFound && saveFound
	}); err != nil {
		t.Fatalf("wait for ignored hotkey app editor: %v", err)
	}
	if err := client.Perform(ctx, "form-table-row-field-0", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("open ignored hotkey app picker: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, dialogFound := automationdriver.Find(snapshot, "form-table-app-dialog")
		_, searchFound := automationdriver.Find(snapshot, "form-table-app-search")
		return dialogFound && searchFound
	}); err != nil {
		t.Fatalf("wait for ignored hotkey app picker: %v", err)
	}
	if err := client.Perform(ctx, "form-table-app-search", woxui.AccessibilityActionSetValue, query); err != nil {
		t.Fatalf("search ignored hotkey apps for %q: %v", query, err)
	}
	candidateID := waitForSingleIgnoredAppCandidate(t, ctx, client)
	if err := client.Perform(ctx, candidateID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("select ignored hotkey app %q: %v", query, err)
	}
	if err := client.Perform(ctx, "form-table-app-confirm", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("confirm ignored hotkey app %q: %v", query, err)
	}
	if err := client.Perform(ctx, "form-table-row-save", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("save ignored hotkey app %q: %v", query, err)
	}
	rowDeleteID := ignoredHotkeyAppsFieldID + "-row-" + strconv.Itoa(rowIndex) + "-delete"
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, rowFound := automationdriver.Find(snapshot, rowDeleteID)
		add, addFound := automationdriver.Find(snapshot, ignoredHotkeyAppsFieldID+"-add")
		return rowFound && addFound && add.Enabled
	}); err != nil {
		t.Fatalf("wait for ignored hotkey app %q to persist: %v", query, err)
	}
	return rowIndex
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

// ignoredHotkeyAppRowCount returns the number of persisted rows visible in the inline table.
func ignoredHotkeyAppRowCount(t *testing.T, ctx context.Context, client *automationdriver.Client) int {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read ignored hotkey app rows: %v", err)
	}
	count := 0
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, ignoredHotkeyAppsFieldID+"-row-") && strings.HasSuffix(node.AutomationID, "-delete") {
			count++
		}
	}
	return count
}

// waitForSingleIgnoredAppCandidate resolves the one platform candidate remaining after identity search.
func waitForSingleIgnoredAppCandidate(t *testing.T, ctx context.Context, client *automationdriver.Client) string {
	t.Helper()
	var candidateID string
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		candidateID = ""
		count := 0
		for _, node := range snapshot.Tree.Nodes {
			if node.Role == woxui.AccessibilityRoleRadioButton && strings.HasPrefix(node.AutomationID, "form-table-app-") {
				candidateID = node.AutomationID
				count++
			}
		}
		return count == 1
	}); err != nil {
		t.Fatalf("wait for one ignored hotkey app candidate: %v", err)
	}
	return candidateID
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
	rowDeleteID := ignoredHotkeyAppsFieldID + "-row-" + strconv.Itoa(rowIndex) + "-delete"
	if err := client.Perform(ctx, rowDeleteID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("delete ignored hotkey app row %d: %v", rowIndex, err)
	}
	if err := client.Perform(ctx, "form-table-delete-confirm", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("confirm ignored hotkey app row %d deletion: %v", rowIndex, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, rowFound := automationdriver.Find(snapshot, rowDeleteID)
		add, addFound := automationdriver.Find(snapshot, ignoredHotkeyAppsFieldID+"-add")
		return !rowFound && addFound && add.Enabled
	}); err != nil {
		t.Fatalf("wait for ignored hotkey app row %d removal: %v", rowIndex, err)
	}
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
