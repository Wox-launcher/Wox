//go:build wox_ui_smoke

package data

import (
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

// Test001SettingDataLogLevel verifies that the Data page log level controls real query logging.
func Test001SettingDataLogLevel(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		logPath := filepath.Join(os.Getenv(automationdriver.SharedDataDirectoryEnvironment), "log", "wox.log")
		smoke.ShowLauncher(t, ctx, client)
		openDataSettings(t, ctx, client)
		setLogLevel(t, ctx, client, "DEBUG")
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("close Data settings after selecting DEBUG: %v", err)
		}

		smoke.ShowLauncher(t, ctx, client)
		debugOffset := currentLogSize(t, logPath)
		runQueries(t, ctx, client, []string{"smoke-debug-one", "smoke-debug-two", "smoke-debug-three"})
		debugLogs := waitForLog(t, ctx, logPath, debugOffset, func(logs string) bool {
			return strings.Contains(logs, "smoke-debug-three") && strings.Contains(logs, "[DBG]")
		})
		if !strings.Contains(debugLogs, "[INF]") {
			t.Fatalf("DEBUG query logs contain no INFO entry:\n%s", debugLogs)
		}

		if err := client.OpenSettings(ctx, "/data"); err != nil {
			t.Fatalf("reopen Data settings: %v", err)
		}
		waitForDataSettings(t, ctx, client)
		setLogLevel(t, ctx, client, "INFO")
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("close Data settings after selecting INFO: %v", err)
		}

		smoke.ShowLauncher(t, ctx, client)
		infoOffset := currentLogSize(t, logPath)
		runQueries(t, ctx, client, []string{"smoke-info-one", "smoke-info-two", "smoke-info-three"})
		infoLogs := waitForLog(t, ctx, logPath, infoOffset, func(logs string) bool {
			return strings.Contains(logs, "smoke-info-three") && strings.Contains(logs, "[INF]")
		})
		if strings.Contains(infoLogs, "[DBG]") {
			t.Fatalf("INFO query logs unexpectedly contain DEBUG entries:\n%s", infoLogs)
		}
	})
}

func openDataSettings(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.OpenSettings(ctx, "/data"); err != nil {
		t.Fatalf("open Data settings: %v", err)
	}
	waitForDataSettings(t, ctx, client)
}

// waitForDataSettings waits until the native Data page and its log-level control are actionable.
func waitForDataSettings(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, windowFound := automationdriver.Find(snapshot, "settings.window")
		_, pageFound := automationdriver.Find(snapshot, "settings.page.data")
		_, logLevelFound := automationdriver.Find(snapshot, "data-log-level")
		return windowFound && pageFound && logLevelFound
	}); err != nil {
		t.Fatalf("wait for Data settings: %v", err)
	}
}

// setLogLevel selects a log level through the visible dropdown and waits for persistence.
func setLogLevel(t *testing.T, ctx context.Context, client *automationdriver.Client, expected string) {
	t.Helper()
	smoke.SelectSettingChoiceByLabel(t, ctx, client, "data-log-level", expected)
}

// runQueries drives several launcher changes and waits for the final input to reconcile.
func runQueries(t *testing.T, ctx context.Context, client *automationdriver.Client, queries []string) {
	t.Helper()
	for _, query := range queries {
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
			t.Fatalf("enter query %q: %v", query, err)
		}
	}
	want := queries[len(queries)-1]
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		node, found := automationdriver.Find(snapshot, "launcher.query.input")
		return found && node.Value == want
	}); err != nil {
		t.Fatalf("wait for query %q: %v", want, err)
	}
}

func currentLogSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat Wox log %q: %v", path, err)
	}
	return info.Size()
}

// waitForLog tails only entries written after offset until the expected query activity appears.
func waitForLog(t *testing.T, ctx context.Context, path string, offset int64, matches func(string) bool) string {
	t.Helper()
	data := smoke.WaitForFile(t, ctx, path, func(data []byte) bool {
		if int64(len(data)) >= offset {
			logs := string(data[offset:])
			return matches(logs)
		}
		return false
	})
	return string(data[offset:])
}
