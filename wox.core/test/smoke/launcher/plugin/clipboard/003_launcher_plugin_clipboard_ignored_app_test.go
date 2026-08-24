//go:build wox_ui_smoke

package clipboard

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	woxclipboard "wox/util/clipboard"
)

const clipboardPluginID = "5f815d98-27f5-488d-a756-c317ea39935b"

// Test003LauncherPluginClipboardIgnoredApp verifies ignored applications bypass Clipboard history.
// Flow: add the platform editor to Clipboard privacy settings -> copy unique text in that editor -> query Clipboard.
// Evidence: a fresh ignore log identifies the source application and the completed Clipboard query omits the marker.
func Test003LauncherPluginClipboardIgnoredApp(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		requireClipboardIgnoredAppRuntime(t)
		smoke.PreserveClipboard(t)

		logPath := filepath.Join(os.Getenv(automationdriver.SharedDataDirectoryEnvironment), "log", "wox.log")
		smoke.WaitForApplicationCatalog(t, ctx, logPath)
		appQuery, appIdentity := ignoredClipboardAppTarget(t)
		fieldID := openClipboardIgnoredApplications(t, ctx, client)
		rowIndex := smoke.AddApplicationTableRow(t, ctx, client, fieldID, appQuery)
		t.Cleanup(func() { removeIgnoredClipboardApp(t, client, rowIndex) })
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("close Clipboard settings: %v", err)
		}

		marker := fmt.Sprintf("wox-smoke-private-clipboard-%d", time.Now().UnixNano())
		logOffset := currentLogSize(t, logPath)
		copyTextFromIgnoredApplication(t, ctx, marker)
		waitForSystemClipboardText(t, ctx, marker)
		smoke.WaitForFile(t, ctx, logPath, func(data []byte) bool {
			if logOffset > int64(len(data)) {
				return false
			}
			freshLogs := string(data[logOffset:])
			return strings.Contains(freshLogs, "clipboard: ignored change from app identity="+appIdentity)
		})

		smoke.ShowLauncher(t, ctx, client)
		snapshot := queryClipboardWithoutMarker(t, ctx, client, marker)
		if _, found := smoke.FindLauncherResult(snapshot, marker); found {
			t.Fatalf("ignored clipboard text %q was persisted in Clipboard history", marker)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

// queryClipboardWithoutMarker waits for either an empty or completed Clipboard result collection.
func queryClipboardWithoutMarker(t *testing.T, ctx context.Context, client *automationdriver.Client, marker string) woxwidget.AutomationSnapshot {
	t.Helper()
	query := "cb " + marker
	if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
		t.Fatalf("set Clipboard query %q: %v", query, err)
	}
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
		if !inputFound || input.Value != query {
			return false
		}
		_, loading := automationdriver.Find(snapshot, "launcher.query.loading")
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		return !loading && (!resultsFound || results.Value == "complete")
	})
	if err != nil {
		t.Fatalf("wait for Clipboard query %q: %v", query, err)
	}
	return snapshot
}

// openClipboardIgnoredApplications opens Clipboard settings and resolves its application table field.
func openClipboardIgnoredApplications(t *testing.T, ctx context.Context, client *automationdriver.Client) string {
	t.Helper()
	smoke.OpenInstalledPluginSettings(t, ctx, client, clipboardPluginID)
	fieldID := ""
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		fieldID = ""
		count := 0
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "plugin-settings-field-") && strings.HasSuffix(node.AutomationID, "-add") {
				fieldID = strings.TrimSuffix(node.AutomationID, "-add")
				count++
			}
		}
		return count == 1
	}); err != nil {
		t.Fatalf("wait for Clipboard ignored applications table: %v", err)
	}
	return fieldID
}

// removeIgnoredClipboardApp removes the row created by this case through the real Settings UI.
func removeIgnoredClipboardApp(t *testing.T, client *automationdriver.Client, rowIndex int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), automationdriver.ActionTimeout)
	defer cancel()
	if err := client.Hide(ctx); err != nil {
		t.Errorf("hide active window before removing ignored Clipboard app: %v", err)
	}
	fieldID := openClipboardIgnoredApplications(t, ctx, client)
	smoke.RemoveApplicationTableRow(t, ctx, client, fieldID, rowIndex)
	if err := client.Hide(ctx); err != nil {
		t.Errorf("close Clipboard settings after cleanup: %v", err)
	}
}

// currentLogSize captures the boundary before the ignored clipboard event is produced.
func currentLogSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("read Wox log size: %v", err)
	}
	return info.Size()
}

// waitForSystemClipboardText proves the native editor completed the requested copy operation.
func waitForSystemClipboardText(t *testing.T, ctx context.Context, expected string) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		actual, err := woxclipboard.ReadText()
		if err == nil && actual == expected {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for system clipboard text %q: %v", expected, ctx.Err())
		case <-ticker.C:
		}
	}
}
