//go:build wox_ui_smoke

package shell

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test005LauncherQueryShellTerminalSearch verifies terminal output search, highlighted navigation, and dismissal.
// Flow: execute output with two matches -> open find -> search -> next -> previous -> close the find bar.
// Evidence: the live highlighted-match state changes 1/2 -> 2/2 -> 1/2 before the search UI disappears.
func Test005LauncherQueryShellTerminalSearch(t *testing.T) {
	const (
		command = "echo woxfind; echo woxfind"
		query   = "woxfind"
	)

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, "> "+command); err != nil {
			t.Fatalf("enter searchable Shell query: %v", err)
		}
		resultID := waitForShellResult(t, ctx, client, command)
		if err := client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("execute searchable Shell result: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, statusFound := automationdriver.Find(snapshot, "launcher.preview.terminal.status")
			output, outputFound := automationdriver.Find(snapshot, "launcher.preview.terminal.output")
			return statusFound && status.Value == "completed" && outputFound && strings.Count(output.Value, query) == 2
		}); err != nil {
			t.Fatalf("wait for searchable terminal output: %v", err)
		}

		modifier := woxui.KeyModifierControl
		if runtime.GOOS == "darwin" {
			modifier = woxui.KeyModifierMeta
		}
		if err := client.PressKey(ctx, woxui.Key("f"), modifier|woxui.KeyModifierShift); err != nil {
			t.Fatalf("open terminal search: %v", err)
		}
		searchID, snapshot := waitForTerminalControl(t, ctx, client, "terminal-search-input-")
		if err := client.Perform(ctx, searchID, woxui.AccessibilityActionSetValue, query); err != nil {
			t.Fatalf("set terminal search query: %v", err)
		}
		snapshot = waitForTerminalMatchCount(t, ctx, client, "1/2")
		assertShellSnapshot(t, snapshot)

		nextID, _ := terminalControl(snapshot, "terminal-search-next-")
		if nextID == "" {
			t.Fatal("terminal next-match action is missing")
		}
		if err := client.Perform(ctx, nextID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("move to next terminal match: %v", err)
		}
		snapshot = waitForTerminalMatchCount(t, ctx, client, "2/2")

		previousID, _ := terminalControl(snapshot, "terminal-search-previous-")
		if previousID == "" {
			t.Fatal("terminal previous-match action is missing")
		}
		if err := client.Perform(ctx, previousID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("move to previous terminal match: %v", err)
		}
		snapshot = waitForTerminalMatchCount(t, ctx, client, "1/2")

		closeID, _ := terminalControl(snapshot, "terminal-search-close-")
		if closeID == "" {
			t.Fatal("terminal close-search action is missing")
		}
		if err := client.Perform(ctx, closeID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("close terminal search: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, inputFound := terminalControl(snapshot, "terminal-search-input-")
			_, countFound := automationdriver.Find(snapshot, "launcher.preview.terminal.search.match-count")
			return !inputFound && !countFound
		}); err != nil {
			t.Fatalf("wait for terminal search to close: %v", err)
		}
	})
}

// waitForTerminalControl waits for one dynamic terminal-session control ID.
func waitForTerminalControl(t *testing.T, ctx context.Context, client *automationdriver.Client, prefix string) (string, woxwidget.AutomationSnapshot) {
	t.Helper()
	var controlID string
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		controlID, _ = terminalControl(snapshot, prefix)
		return controlID != ""
	})
	if err != nil {
		t.Fatalf("wait for terminal control %q: %v", prefix, err)
	}
	return controlID, snapshot
}

// waitForTerminalMatchCount waits for navigation to expose the selected match index.
func waitForTerminalMatchCount(t *testing.T, ctx context.Context, client *automationdriver.Client, value string) woxwidget.AutomationSnapshot {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		count, found := automationdriver.Find(snapshot, "launcher.preview.terminal.search.match-count")
		return found && count.Value == value
	})
	if err != nil {
		t.Fatalf("wait for terminal match count %q: %v", value, err)
	}
	return snapshot
}

// terminalControl resolves the current session's dynamic control ID by its stable action prefix.
func terminalControl(snapshot woxwidget.AutomationSnapshot, prefix string) (string, bool) {
	node, found := automationdriver.FindByAutomationIDPrefix(snapshot, prefix)
	return node.AutomationID, found
}
