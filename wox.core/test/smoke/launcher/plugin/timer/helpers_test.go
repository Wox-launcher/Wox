//go:build wox_ui_smoke

package timer

import (
	"context"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const timerOverlayInstancePrefix = "overlay.wox_timer_overlay_"

// startPinnedTimer creates a timer through its default Launcher action and resolves the durable timer ID from a fresh list query.
func startPinnedTimer(t *testing.T, ctx context.Context, client *automationdriver.Client, duration, note string) string {
	t.Helper()
	snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, "timer "+duration+" "+note)
	var startResult woxui.AccessibilityNode
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && node.Selected && node.Description == note {
			startResult = node
			break
		}
	}
	if startResult.AutomationID == "" {
		t.Fatalf("Timer start result for %q was not selected", note)
	}
	if err := client.Perform(ctx, startResult.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("start pinned Timer %q: %v", note, err)
	}
	if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
		return state.Exists && !state.Visible
	}); err != nil {
		t.Fatalf("wait for Launcher to hide after starting Timer %q: %v", note, err)
	}

	if err := client.FocusInstance(ctx, "primary"); err != nil {
		t.Fatalf("focus primary Launcher after starting Timer: %v", err)
	}
	smoke.ShowLauncher(t, ctx, client)
	snapshot = smoke.ReplaceLauncherQuery(t, ctx, client, "timer list")
	result, found := timerResultByNote(snapshot, note)
	if !found {
		t.Fatalf("active Timer %q was not found", note)
	}
	return strings.TrimPrefix(result.AutomationID, "launcher.result.")
}

func timerResultByNote(snapshot woxwidget.AutomationSnapshot, note string) (woxui.AccessibilityNode, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && node.Description == note {
			return node, true
		}
	}
	return woxui.AccessibilityNode{}, false
}

// selectTimerByNote refreshes the Timer query and moves keyboard selection to the uniquely named active timer.
func selectTimerByNote(t *testing.T, ctx context.Context, client *automationdriver.Client, note string) (woxui.AccessibilityNode, woxwidget.AutomationSnapshot) {
	t.Helper()
	snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, "timer list")
	result, found := timerResultByNote(snapshot, note)
	if !found {
		t.Fatalf("active Timer %q was not found", note)
	}
	snapshot = smoke.SelectLauncherResult(t, ctx, client, result.AutomationID)
	result, found = timerResultByNote(snapshot, note)
	if !found || !result.Selected {
		t.Fatalf("active Timer %q was not selected", note)
	}
	return result, snapshot
}

// deleteTimer removes an active timer through its result action and waits for the refreshed query to drop it.
func deleteTimer(t *testing.T, ctx context.Context, client *automationdriver.Client, note string) {
	t.Helper()
	if err := client.FocusInstance(ctx, "primary"); err != nil {
		t.Fatalf("focus primary Launcher before deleting Timer: %v", err)
	}
	smoke.ShowLauncher(t, ctx, client)
	result, _ := selectTimerByNote(t, ctx, client, note)
	timerID := strings.TrimPrefix(result.AutomationID, "launcher.result.")
	smoke.ActivateSelectedResultAction(t, ctx, client, "action-result-"+timerID+":delete-")
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := timerResultByNote(snapshot, note)
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		return resultsFound && results.Value == "complete" && !found
	}); err != nil {
		t.Fatalf("wait for Timer %q deletion: %v", note, err)
	}
}

// cleanupTimer protects the shared smoke process from a timer left behind by an earlier assertion failure.
func cleanupTimer(t *testing.T, client *automationdriver.Client, note string) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), automationdriver.ActionTimeout)
		defer cancel()
		if err := client.FocusInstance(ctx, "primary"); err != nil {
			t.Errorf("focus primary Launcher during Timer cleanup: %v", err)
			return
		}
		if err := client.Show(ctx); err != nil {
			t.Errorf("show Launcher during Timer cleanup: %v", err)
			return
		}
		snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, "timer list")
		result, found := timerResultByNote(snapshot, note)
		if !found {
			return
		}
		snapshot = smoke.SelectLauncherResult(t, ctx, client, result.AutomationID)
		result, found = timerResultByNote(snapshot, note)
		if !found {
			return
		}
		timerID := strings.TrimPrefix(result.AutomationID, "launcher.result.")
		smoke.ActivateSelectedResultAction(t, ctx, client, "action-result-"+timerID+":delete-")
	})
}

func countdownSeconds(value string) (int, bool) {
	parsed, err := time.Parse("04:05", value)
	if err != nil {
		return 0, false
	}
	return parsed.Minute()*60 + parsed.Second(), true
}
